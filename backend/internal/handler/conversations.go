package handler

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"
	"unicode/utf8"

	"boodou/backend/internal/storage"
	"boodou/backend/internal/store"

	"github.com/gin-gonic/gin"
)

const maxTitleRunes = 120

type Conversations struct {
	store *store.Store
	files storage.Storage
}

func NewConversations(st *store.Store, files storage.Storage) *Conversations {
	return &Conversations{store: st, files: files}
}

func (h *Conversations) List(c *gin.Context) {
	list, err := h.store.ListConversations(c.Request.Context())
	if err != nil {
		log.Printf("conversations: list: %v", err)
		jsonError(c, http.StatusInternalServerError, "Daftar percakapan gagal dimuat.")
		return
	}
	if list == nil {
		list = []store.Conversation{}
	}
	c.JSON(http.StatusOK, list)
}

type titleBody struct {
	Title string `json:"title"`
}

func cleanTitle(raw string) (string, bool) {
	t := strings.Join(strings.Fields(raw), " ")
	return t, utf8.RuneCountInString(t) <= maxTitleRunes
}

func (h *Conversations) Create(c *gin.Context) {
	var body titleBody
	// The body is optional; an empty title is filled from the first message.
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&body); err != nil {
			jsonError(c, http.StatusBadRequest, "Format permintaan tidak valid.")
			return
		}
	}
	title, ok := cleanTitle(body.Title)
	if !ok {
		jsonError(c, http.StatusBadRequest, "Judul paling panjang 120 karakter.")
		return
	}
	conv, err := h.store.CreateConversation(c.Request.Context(), title)
	if err != nil {
		log.Printf("conversations: create: %v", err)
		jsonError(c, http.StatusInternalServerError, "Percakapan gagal dibuat.")
		return
	}
	c.JSON(http.StatusCreated, conv)
}

func (h *Conversations) Get(c *gin.Context) {
	conv, msgs, err := h.store.GetConversation(c.Request.Context(), c.Param("id"))
	if errors.Is(err, store.ErrNotFound) {
		jsonError(c, http.StatusNotFound, "Percakapan tidak ditemukan.")
		return
	}
	if err != nil {
		log.Printf("conversations: get: %v", err)
		jsonError(c, http.StatusInternalServerError, "Percakapan gagal dimuat.")
		return
	}
	if msgs == nil {
		msgs = []store.Message{}
	}
	c.JSON(http.StatusOK, gin.H{
		"id": conv.ID, "title": conv.Title, "created_at": conv.CreatedAt, "updated_at": conv.UpdatedAt,
		"messages": msgs,
	})
}

func (h *Conversations) Rename(c *gin.Context) {
	var body titleBody
	if err := c.ShouldBindJSON(&body); err != nil {
		jsonError(c, http.StatusBadRequest, "Format permintaan tidak valid.")
		return
	}
	title, ok := cleanTitle(body.Title)
	if title == "" {
		jsonError(c, http.StatusBadRequest, "Judul tidak boleh kosong.")
		return
	}
	if !ok {
		jsonError(c, http.StatusBadRequest, "Judul paling panjang 120 karakter.")
		return
	}
	conv, err := h.store.RenameConversation(c.Request.Context(), c.Param("id"), title)
	if errors.Is(err, store.ErrNotFound) {
		jsonError(c, http.StatusNotFound, "Percakapan tidak ditemukan.")
		return
	}
	if err != nil {
		log.Printf("conversations: rename: %v", err)
		jsonError(c, http.StatusInternalServerError, "Judul gagal disimpan.")
		return
	}
	c.JSON(http.StatusOK, conv)
}

func (h *Conversations) Delete(c *gin.Context) {
	keys, err := h.store.DeleteConversation(c.Request.Context(), c.Param("id"))
	if errors.Is(err, store.ErrNotFound) {
		jsonError(c, http.StatusNotFound, "Percakapan tidak ditemukan.")
		return
	}
	if err != nil {
		log.Printf("conversations: delete: %v", err)
		jsonError(c, http.StatusInternalServerError, "Percakapan gagal dihapus.")
		return
	}
	// The rows are already gone, so finish removing files even if the client
	// disconnects now.
	removeFiles(context.WithoutCancel(c.Request.Context()), h.files, keys)
	c.Status(http.StatusNoContent)
}
