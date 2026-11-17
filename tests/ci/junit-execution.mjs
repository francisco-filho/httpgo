// Validate actual CLI artifacts with the same parser used to publish in CI.
import assert from 'node:assert/strict'
import {readFile} from 'node:fs/promises'
import {resolve, join} from 'node:path'
import {pathToFileURL} from 'node:url'

const [consumer, directory] = process.argv.slice(2)
const {JavaJunitParser} = await import(pathToFileURL(resolve(consumer, 'lib/parsers/java-junit/java-junit-parser.js')))
const parser = new JavaJunitParser({trackedFiles: [], parseErrors: true})
const expected = {
  success: ['success', 'success', 'success', 'success'],
  mixed: ['success', 'failed', 'failed', 'success'],
  'fail-fast': ['success', 'failed', 'skipped', 'skipped'],
  'transport-fast': ['success', 'success', 'failed', 'skipped']
}
for (const [mode, states] of Object.entries(expected)) {
  const path = join(directory, `${mode}.xml`)
  const result = await parser.parse(path, await readFile(path, 'utf8'))
  assert.equal(result.tests, 4, mode)
  const cases = result.suites.flatMap(suite => suite.groups.flatMap(group => group.tests))
  assert.deepEqual(cases.map(test => test.result), states, mode)
  assert.ok(cases[0].name.endsWith('01-ação & "aspas" <ok>.hurl'), mode)
  assert.ok(Number.isFinite(result.time) && result.time >= 0, mode)
  for (const test of cases) {
    assert.ok(Number.isFinite(test.time) && test.time >= 0, mode)
    if (test.result === 'failed') {
      assert.match(test.error.details, /\.hurl":\d+:\d+:/, mode)
    }
    if (test.result === 'skipped') assert.equal(test.time, 0, mode)
  }
  console.log(`CLI ${mode}: ${result.passed} passed, ${result.failed} failure/error, ${result.skipped} skipped`)
}
