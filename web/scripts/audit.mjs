// CI dependency gate.
//
// `npm audit` cannot know how an advisory applies to us, so anything we have
// deliberately accepted is listed here with its reason and an expiry. Anything
// else at high or critical fails the build (docs/12, A06).
import { execSync } from 'node:child_process'

const accepted = {
  'GHSA-qwww-vcr4-c8h2': {
    package: 'react-router',
    reason:
      'RSC-mode CSRF bypass. ruuma ships a client-only SPA: no React Router ' +
      'server/framework mode, no RSC, no server actions, so the affected code ' +
      'path does not exist in our build. Every other version of react-router 7 ' +
      'carries a longer list of advisories, so this is the safest available pin.',
    review: '2026-11-01',
  },
}

// npm audit exits non-zero whenever it finds anything, so the report has to be
// read from the thrown error's stdout as well as from a clean run.
let raw
try {
  raw = execSync('npm audit --json', { encoding: 'utf8', stdio: ['ignore', 'pipe', 'ignore'] })
} catch (err) {
  raw = err.stdout
}
if (!raw) {
  console.error('npm audit: no report produced')
  process.exit(1)
}
const report = JSON.parse(raw)
const vulns = Object.values(report.vulnerabilities ?? {})

// A package's `via` is either an advisory object or the name of another
// vulnerable package, so the ids have to be resolved through those links —
// otherwise a package that is only vulnerable through an accepted dependency
// blocks the build as "transitive" with nothing actionable behind it.
const byName = new Map(vulns.map((v) => [v.name, v]))

function advisoryIds(vuln, seen = new Set()) {
  if (seen.has(vuln.name)) return []
  seen.add(vuln.name)

  const ids = []
  for (const entry of vuln.via ?? []) {
    if (typeof entry === 'object') {
      const id = entry.url?.split('/').pop()
      if (id) ids.push(id)
    } else if (byName.has(entry)) {
      ids.push(...advisoryIds(byName.get(entry), seen))
    }
  }
  return ids
}

const blocking = []
for (const v of vulns) {
  if (!['high', 'critical'].includes(v.severity)) continue
  const ids = [...new Set(advisoryIds(v))]
  const unaccepted = ids.filter((id) => !accepted[id])
  if (unaccepted.length > 0 || ids.length === 0) {
    blocking.push(`${v.name} (${v.severity}): ${unaccepted.join(', ') || 'no advisory id resolved'}`)
  }
}

if (blocking.length > 0) {
  console.error('npm audit: unaccepted high/critical advisories:')
  for (const line of blocking) console.error('  - ' + line)
  process.exit(1)
}

const today = new Date().toISOString().slice(0, 10)
for (const [id, entry] of Object.entries(accepted)) {
  if (today > entry.review) {
    console.error(`npm audit: accepted advisory ${id} (${entry.package}) is past its review date ${entry.review}`)
    process.exit(1)
  }
  console.log(`npm audit: accepting ${id} (${entry.package}) until ${entry.review}`)
}
console.log('npm audit: no unaccepted high or critical advisories')
