// sharp/librsvg, which renders the PWA icons, ignores CSS custom properties,
// so every var() fill comes out black. public/mascot.svg stays untouched for
// browsers; this renders from a copy with the variables already resolved.
import { execFileSync } from 'node:child_process'
import { copyFileSync, mkdirSync, readdirSync, readFileSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'

const SRC = 'public/mascot.svg'
const WORK = '.icon-build'

const svg = readFileSync(SRC, 'utf8')

// Collect the private --_* tokens declared on .ghost; outside the app the
// public --ghost-* inputs are never set, so each resolves to its fallback.
const decls = new Map()
for (const [, name, value] of svg.matchAll(/(--_[\w-]+):\s*([^;]+);/g)) decls.set(name, value.trim())

function resolve(value, depth = 0) {
  if (depth > 10) throw new Error(`Variabel CSS melingkar: ${value}`)
  const next = value.replace(/var\((--[\w-]+)(?:,\s*((?:[^()]|\([^()]*\))*))?\)/g, (_, name, fallback) => {
    if (decls.has(name)) return decls.get(name)
    if (fallback !== undefined) return fallback.trim()
    throw new Error(`Variabel ${name} tidak punya nilai di ${SRC}`)
  })
  return next.includes('var(') ? resolve(next, depth + 1) : next
}

for (const [name, value] of decls) decls.set(name, resolve(value))
const flat = svg.replace(/var\([^;{}]*\)(?=\s*[;}])/g, (expr) => resolve(expr))
if (flat.includes('var(')) throw new Error('Masih ada var() yang belum terselesaikan')

mkdirSync(WORK, { recursive: true })
writeFileSync(join(WORK, 'mascot.svg'), flat)

execFileSync(process.execPath, ['node_modules/@vite-pwa/assets-generator/bin/pwa-assets-generator.mjs'], {
  stdio: 'inherit',
})

for (const file of readdirSync(WORK)) {
  if (/\.(png|ico)$/.test(file)) copyFileSync(join(WORK, file), join('public', file))
}
console.log(`Ikon disalin ke public/ dari ${SRC}`)
