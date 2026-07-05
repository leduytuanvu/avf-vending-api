const fs = require('fs');
const path = require('path');

function walkDir(dir, results = []) {
  const entries = fs.readdirSync(dir, { withFileTypes: true });
  for (const e of entries) {
    const full = path.join(dir, e.name);
    if (e.isDirectory()) walkDir(full, results);
    else if (e.name.endsWith('.request.yaml')) results.push(full);
  }
  return results;
}

function extractScripts(content) {
  const scripts = [];
  // Match prerequest or tests block (YAML block scalar |- or |)
  const re = /^( *)(prerequest|tests):\s*\|[-+]?\s*\n((?:\1 +.*\n?)*)/gm;
  let m;
  while ((m = re.exec(content)) !== null) {
    const type = m[2];
    const raw = m[3];
    // De-indent: find minimum indent of non-empty lines
    const lines = raw.split('\n');
    const minIndent = lines
      .filter(l => l.trim().length > 0)
      .reduce((min, l) => Math.min(min, l.match(/^ */)[0].length), Infinity);
    const code = lines.map(l => l.slice(minIndent)).join('\n');
    scripts.push({ type, code, offset: m.index });
  }
  return scripts;
}

function checkSyntax(code) {
  try {
    new Function(code);
    return null;
  } catch (e) {
    return e.message;
  }
}

const collectionsDir = path.join(__dirname, 'collections');
const files = walkDir(collectionsDir);
const errors = [];

for (const file of files) {
  const content = fs.readFileSync(file, 'utf8');
  const scripts = extractScripts(content);
  for (const { type, code } of scripts) {
    const err = checkSyntax(code);
    if (err) {
      errors.push({ file: path.relative(collectionsDir, file), type, error: err, code });
    }
  }
}

if (errors.length === 0) {
  console.log('No syntax errors found in any scripts.');
} else {
  console.log(`Found ${errors.length} syntax error(s):\n`);
  for (const e of errors) {
    console.log(`FILE: ${e.file}`);
    console.log(`SCRIPT TYPE: ${e.type}`);
    console.log(`ERROR: ${e.error}`);
    console.log(`--- CODE SNIPPET (first 20 lines) ---`);
    console.log(e.code.split('\n').slice(0, 20).join('\n'));
    console.log('---\n');
  }
}
