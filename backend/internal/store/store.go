// Package store is the PostgreSQL repository for conversations, messages
// and attachments.
package store

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound = errors.New("store: tidak ditemukan")
	// ErrAttachmentUnavailable means an id is unknown or already used by
	// another message; reusing one would let two messages share a file.
	ErrAttachmentUnavailable = errors.New("store: lampiran tidak tersedia")
)

const (
	RoleUser      = "user"
	RoleAssistant = "assistant"

	StatusComplete = "complete"
	StatusStopped  = "stopped"
	StatusError    = "error"
)

type Conversation struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Attachment struct {
	ID         string    `json:"id"`
	MIME       string    `json:"mime"`
	Width      int       `json:"width"`
	Height     int       `json:"height"`
	SizeBytes  int       `json:"-"`
	SHA256     string    `json:"-"`
	StorageKey string    `json:"-"`
	CreatedAt  time.Time `json:"-"`
}

type Message struct {
	ID          string       `json:"id"`
	Role        string       `json:"role"`
	Content     string       `json:"content"`
	Status      string       `json:"status"`
	Model       string       `json:"model"`
	CreatedAt   time.Time    `json:"created_at"`
	Attachments []Attachment `json:"attachments"`
}

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// ValidID lets callers turn a malformed id into a 404 instead of a SQL error.
func ValidID(id string) bool {
	return uuidPattern.MatchString(id)
}

type Store struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) CreateConversation(ctx context.Context, title string) (Conversation, error) {
	var c Conversation
	err := s.pool.QueryRow(ctx,
		`INSERT INTO conversations (title) VALUES ($1) RETURNING id, title, created_at, updated_at`, title,
	).Scan(&c.ID, &c.Title, &c.CreatedAt, &c.UpdatedAt)
	return c, err
}

func (s *Store) ListConversations(ctx context.Context) ([]Conversation, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, title, created_at, updated_at FROM conversations ORDER BY updated_at DESC, id`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Conversation, error) {
		var c Conversation
		err := r.Scan(&c.ID, &c.Title, &c.CreatedAt, &c.UpdatedAt)
		return c, err
	})
}

func (s *Store) GetConversation(ctx context.Context, id string) (Conversation, []Message, error) {
	if !ValidID(id) {
		return Conversation{}, nil, ErrNotFound
	}
	var c Conversation
	err := s.pool.QueryRow(ctx,
		`SELECT id, title, created_at, updated_at FROM conversations WHERE id = $1`, id,
	).Scan(&c.ID, &c.Title, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Conversation{}, nil, ErrNotFound
	}
	if err != nil {
		return Conversation{}, nil, err
	}
	msgs, err := s.Messages(ctx, id)
	return c, msgs, err
}

// Messages returns the conversation in order, each with its attachments.
func (s *Store) Messages(ctx context.Context, conversationID string) ([]Message, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, role, content, status, model, created_at
		   FROM messages WHERE conversation_id = $1 ORDER BY created_at, id`, conversationID)
	if err != nil {
		return nil, err
	}
	msgs, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (Message, error) {
		var m Message
		err := r.Scan(&m.ID, &m.Role, &m.Content, &m.Status, &m.Model, &m.CreatedAt)
		m.Attachments = []Attachment{}
		return m, err
	})
	if err != nil {
		return nil, err
	}

	rows, err = s.pool.Query(ctx,
		`SELECT a.message_id, a.id, a.mime, a.width, a.height, a.size_bytes, a.sha256, a.storage_key, a.created_at
		   FROM attachments a JOIN messages m ON m.id = a.message_id
		  WHERE m.conversation_id = $1 ORDER BY a.position NULLS LAST, a.created_at, a.id`, conversationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	index := make(map[string]int, len(msgs))
	for i, m := range msgs {
		index[m.ID] = i
	}
	for rows.Next() {
		var msgID string
		var a Attachment
		if err := rows.Scan(&msgID, &a.ID, &a.MIME, &a.Width, &a.Height, &a.SizeBytes, &a.SHA256, &a.StorageKey, &a.CreatedAt); err != nil {
			return nil, err
		}
		if i, ok := index[msgID]; ok {
			msgs[i].Attachments = append(msgs[i].Attachments, a)
		}
	}
	return msgs, rows.Err()
}

func (s *Store) RenameConversation(ctx context.Context, id, title string) (Conversation, error) {
	if !ValidID(id) {
		return Conversation{}, ErrNotFound
	}
	var c Conversation
	err := s.pool.QueryRow(ctx,
		`UPDATE conversations SET title = $2, updated_at = now() WHERE id = $1
		 RETURNING id, title, created_at, updated_at`, id, title,
	).Scan(&c.ID, &c.Title, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Conversation{}, ErrNotFound
	}
	return c, err
}

// DeleteConversation removes the rows (attachments go by cascade) and returns
// the storage keys so the caller can delete the files afterwards.
func (s *Store) DeleteConversation(ctx context.Context, id string) ([]string, error) {
	if !ValidID(id) {
		return nil, ErrNotFound
	}
	var keys []string
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx,
			`SELECT a.storage_key FROM attachments a JOIN messages m ON m.id = a.message_id
			  WHERE m.conversation_id = $1`, id)
		if err != nil {
			return err
		}
		keys, err = pgx.CollectRows(rows, pgx.RowTo[string])
		if err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `DELETE FROM conversations WHERE id = $1`, id)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrNotFound
		}
		return nil
	})
	return keys, err
}

// CreateAttachment inserts the row and derives storage_key from the new id,
// so the stored file name never comes from the user.
func (s *Store) CreateAttachment(ctx context.Context, a Attachment, ext string) (Attachment, error) {
	err := s.pool.QueryRow(ctx,
		`WITH n AS (SELECT gen_random_uuid() AS id)
		 INSERT INTO attachments (id, mime, size_bytes, width, height, sha256, storage_key)
		 SELECT n.id, $1, $2, $3, $4, $5, n.id::text || $6 FROM n
		 RETURNING id, storage_key, created_at`,
		a.MIME, a.SizeBytes, a.Width, a.Height, a.SHA256, ext,
	).Scan(&a.ID, &a.StorageKey, &a.CreatedAt)
	return a, err
}

func (s *Store) GetAttachment(ctx context.Context, id string) (Attachment, error) {
	if !ValidID(id) {
		return Attachment{}, ErrNotFound
	}
	var a Attachment
	err := s.pool.QueryRow(ctx,
		`SELECT id, mime, width, height, size_bytes, sha256, storage_key, created_at FROM attachments WHERE id = $1`, id,
	).Scan(&a.ID, &a.MIME, &a.Width, &a.Height, &a.SizeBytes, &a.SHA256, &a.StorageKey, &a.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Attachment{}, ErrNotFound
	}
	return a, err
}

func (s *Store) DeleteAttachment(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM attachments WHERE id = $1`, id)
	return err
}

// AddUserMessage saves the message, claims its attachments, bumps the
// conversation and fills an empty title, all in one transaction so a failed
// claim leaves nothing half-saved.
func (s *Store) AddUserMessage(ctx context.Context, conversationID, text string, attachmentIDs []string) (Message, error) {
	if !ValidID(conversationID) {
		return Message{}, ErrNotFound
	}
	for _, id := range attachmentIDs {
		if !ValidID(id) {
			return Message{}, ErrAttachmentUnavailable
		}
	}

	var m Message
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var title string
		err := tx.QueryRow(ctx, `SELECT title FROM conversations WHERE id = $1 FOR UPDATE`, conversationID).Scan(&title)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}

		err = tx.QueryRow(ctx,
			`INSERT INTO messages (conversation_id, role, content, status) VALUES ($1, 'user', $2, 'complete')
			 RETURNING id, role, content, status, model, created_at`, conversationID, text,
		).Scan(&m.ID, &m.Role, &m.Content, &m.Status, &m.Model, &m.CreatedAt)
		if err != nil {
			return err
		}

		m.Attachments = []Attachment{}
		if len(attachmentIDs) > 0 {
			rows, err := tx.Query(ctx,
				`UPDATE attachments SET message_id = $1, position = array_position($2::uuid[], id)
				 WHERE id = ANY($2::uuid[]) AND message_id IS NULL
				 RETURNING id, mime, width, height, size_bytes, sha256, storage_key, created_at`, m.ID, attachmentIDs)
			if err != nil {
				return err
			}
			claimed, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (Attachment, error) {
				var a Attachment
				err := r.Scan(&a.ID, &a.MIME, &a.Width, &a.Height, &a.SizeBytes, &a.SHA256, &a.StorageKey, &a.CreatedAt)
				return a, err
			})
			if err != nil {
				return err
			}
			if len(claimed) != len(attachmentIDs) {
				return ErrAttachmentUnavailable
			}
			m.Attachments = orderLike(claimed, attachmentIDs)
		}

		if title == "" {
			title = TitleFrom(text, len(attachmentIDs) > 0)
		}
		_, err = tx.Exec(ctx, `UPDATE conversations SET title = $2, updated_at = now() WHERE id = $1`, conversationID, title)
		return err
	})
	if err != nil {
		return Message{}, err
	}
	return m, nil
}

func orderLike(items []Attachment, ids []string) []Attachment {
	byID := make(map[string]Attachment, len(items))
	for _, a := range items {
		byID[a.ID] = a
	}
	out := make([]Attachment, 0, len(ids))
	for _, id := range ids {
		if a, ok := byID[id]; ok {
			out = append(out, a)
		}
	}
	return out
}

func (s *Store) AddAssistantMessage(ctx context.Context, conversationID, content, status, model string) (Message, error) {
	var m Message
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx,
			`INSERT INTO messages (conversation_id, role, content, status, model) VALUES ($1, 'assistant', $2, $3, $4)
			 RETURNING id, role, content, status, model, created_at`, conversationID, content, status, model,
		).Scan(&m.ID, &m.Role, &m.Content, &m.Status, &m.Model, &m.CreatedAt)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE conversations SET updated_at = now() WHERE id = $1`, conversationID)
		return err
	})
	m.Attachments = []Attachment{}
	return m, err
}

// PrepareRetry drops failed answers after the last user message so a retry
// regenerates that answer instead of stacking errors. It reports false when
// there is no user message left to answer.
func (s *Store) PrepareRetry(ctx context.Context, conversationID string) (bool, error) {
	if !ValidID(conversationID) {
		return false, ErrNotFound
	}
	var ok bool
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM conversations WHERE id = $1)`, conversationID).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return ErrNotFound
		}
		_, err := tx.Exec(ctx,
			`DELETE FROM messages WHERE conversation_id = $1 AND role = 'assistant' AND status = 'error'
			   AND created_at > (SELECT max(created_at) FROM messages WHERE conversation_id = $1 AND role = 'user')`,
			conversationID)
		if err != nil {
			return err
		}
		var lastRole string
		err = tx.QueryRow(ctx,
			`SELECT role FROM messages WHERE conversation_id = $1 ORDER BY created_at DESC, id DESC LIMIT 1`, conversationID,
		).Scan(&lastRole)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		ok = lastRole == RoleUser
		return err
	})
	return ok, err
}

// CleanupOrphans deletes attachments never claimed by a message and returns
// their storage keys for file removal.
func (s *Store) CleanupOrphans(ctx context.Context, olderThan time.Duration) ([]string, error) {
	rows, err := s.pool.Query(ctx,
		`DELETE FROM attachments WHERE message_id IS NULL AND created_at < now() - $1::interval RETURNING storage_key`,
		fmt.Sprintf("%d seconds", int(olderThan.Seconds())))
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}
