import { Project, SyntaxKind } from "ts-morph";
import fs from "node:fs";

function die(msg) {
  console.error(msg);
  process.exit(1);
}

const filePath = process.env.FILE_PATH;
const constName = process.env.CONST_NAME;

const vendor = (process.env.VENDOR ?? "").trim();
const date = (process.env.DATE ?? "").trim(); // YYYY-MM-DD
const startUtc = (process.env.START_UTC ?? "").trim(); // HH:MM
const endUtc = (process.env.END_UTC ?? "").trim(); // HH:MM
const tldsRaw = (process.env.TLDS ?? "").trim(); // "Com,Net" (optional)
const useDomainSuffixes = (process.env.USE_DOMAIN_SUFFIXES ?? "false") === "true";

if (!filePath) die("Missing FILE_PATH");
if (!constName) die("Missing CONST_NAME");
if (!vendor || !date || !startUtc || !endUtc) die("Missing required inputs");

const startIso = `${date}T${startUtc}:00Z`;
const endIso = `${date}T${endUtc}:00Z`;

const startDate = new Date(startIso);
const endDate = new Date(endIso);
if (Number.isNaN(startDate.getTime())) die(`Bad start ISO: ${startIso}`);
if (Number.isNaN(endDate.getTime())) die(`Bad end ISO: ${endIso}`);
if (endDate <= startDate) die(`END_UTC must be after START_UTC`);

const tlds = tldsRaw
  ? tldsRaw.split(",").map((t) => t.trim()).filter(Boolean)
  : [];

const project = new Project({
  tsConfigFilePath: undefined,
  skipAddingFilesFromTsConfig: true,
});

const sourceFile = project.addSourceFileAtPath(filePath);
const decl = sourceFile.getVariableDeclaration(constName);
if (!decl) die(`Constant "${constName}" not found in ${filePath}`);

const arrayLiteral = decl.getInitializerIfKind(SyntaxKind.ArrayLiteralExpression);
if (!arrayLiteral) {
  die(`"${constName}" does not have an array initializer (expected: const ${constName} = [ ... ])`);
}

// Helper to extract start ISO from each array element, expecting: start: parseISO('...Z')
function getStartIsoFromElement(el) {
  if (!el.asKind(SyntaxKind.ObjectLiteralExpression)) return null;

  const obj = el.asKindOrThrow(SyntaxKind.ObjectLiteralExpression);
  const startProp = obj.getProperty("start");
  if (!startProp || !startProp.asKind(SyntaxKind.PropertyAssignment)) return null;

  const init = startProp.asKindOrThrow(SyntaxKind.PropertyAssignment).getInitializer();
  if (!init) return null;

  // parseISO('2026-01-01T00:00:00Z')
  if (init.getKind() === SyntaxKind.CallExpression) {
    const call = init.asKindOrThrow(SyntaxKind.CallExpression);
    const arg0 = call.getArguments()[0];
    if (!arg0) return null;
    return arg0.getText().replace(/['"]/g, "");
  }

  return null;
}

// Dedup check: if a window with same start exists, no-op
const elements = arrayLiteral.getElements();
for (const el of elements) {
  const s = getStartIsoFromElement(el);
  if (s === startIso) {
    fs.appendFileSync(process.env.GITHUB_OUTPUT, "changed=false\n");
    process.exit(0);
  }
}

// Build the new object text.
// Two supported shapes:
//
// A) Your real app shape:
//   { start: parseISO('...Z'), end: parseISO('...Z'), maintenanceTlds: [DomainSuffixes.Com] }
//
// B) Simple test shape (no DomainSuffixes):
//   { vendor: 'Verisign', start: '...Z', end: '...Z' }
//
// You choose via USE_DOMAIN_SUFFIXES=true/false.
let newObjectText;

if (useDomainSuffixes) {
  const props = [
    `start: parseISO('${startIso}')`,
    `end: parseISO('${endIso}')`,
  ];

  if (tlds.length) {
    props.push(
      `maintenanceTlds: [${tlds.map((t) => `DomainSuffixes.${t}`).join(", ")}]`
    );
  }

  // Keep a comment for humans
  const hhmm1 = startUtc.replace(":", "");
  const hhmm2 = endUtc.replace(":", "");
  const tldComment = tlds.length ? ` (${tlds.map(t => "." + t.toLowerCase()).join(", ")})` : "";

  newObjectText = `// ${date} ${hhmm1}-${hhmm2} UTC: ${vendor}${tldComment}
{
  ${props.join(",\n  ")}
}`;
} else {
  // Simple “test repo” mode: doesn’t require parseISO/DomainSuffixes imports.
  newObjectText = `{
  vendor: '${vendor}',
  startUtc: parseISO('${startIso}'),
  endUtc: parseISO('${endIso}'),
  tlds: ${JSON.stringify(tlds)}
}`;
}

// Find insertion index by chronological order (based on extracted start ISO)
let insertAt = elements.length;
for (let i = 0; i < elements.length; i++) {
  const s = getStartIsoFromElement(elements[i]);
  if (!s) continue; // skip unknown shapes

  if (s > startIso) {
    insertAt = i;
    break;
  }
}

// Insert into array
arrayLiteral.insertElement(insertAt, newObjectText);

// Save
sourceFile.saveSync();

// Output for PR step
const title = `Maintenance: ${vendor} ${date} ${startUtc}-${endUtc} UTC`;
fs.appendFileSync(process.env.GITHUB_OUTPUT, `changed=true\npr_title=${title}\n`);
