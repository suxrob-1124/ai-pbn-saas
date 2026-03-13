import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";

const readFile = (filePath: string) => readFileSync(filePath, "utf8");

const assertFileContains = (filePath: string, needle: string) => {
  const content = readFile(filePath);
  if (!content.includes(needle)) {
    throw new Error(`missing '${needle}' in ${filePath}`);
  }
};

const root = process.cwd();
const projectPage = path.join(root, "app", "(app)", "projects", "[id]", "page.tsx");
const schedulesSection = path.join(
  root,
  "features",
  "domain-project",
  "components",
  "ProjectSchedulesSection.tsx"
);
const scheduleForm = path.join(root, "components", "ScheduleForm.tsx");
const scheduleRunHistory = path.join(
  root,
  "features",
  "domain-project",
  "components",
  "ScheduleRunHistory.tsx"
);
const scheduleTrigger = path.join(root, "components", "ScheduleTrigger.tsx");

assertFileContains(projectPage, "ProjectSchedulesSection");
assertFileContains(schedulesSection, "ScheduleForm");
assertFileContains(schedulesSection, "ScheduleRunHistory");
assertFileContains(schedulesSection, "Генерация сайтов");
assertFileContains(schedulesSection, "Вставка ссылок (Link Flow)");
assertFileContains(scheduleRunHistory, "История запусков");
assertFileContains(scheduleForm, "Текущее время:");
assertFileContains(scheduleTrigger, "Запуск");

let missingError = false;
try {
  readFile(path.join(root, "components", "MissingSchedule.tsx"));
} catch (err) {
  missingError = true;
  assert.match(String(err), /no such file/i);
}
assert.ok(missingError, "expected missing file error");

console.log("OK");
