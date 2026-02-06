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
const projectPage = path.join(root, "app", "projects", "[id]", "page.tsx");
const scheduleForm = path.join(root, "components", "ScheduleForm.tsx");
const scheduleList = path.join(root, "components", "ScheduleList.tsx");
const scheduleTrigger = path.join(root, "components", "ScheduleTrigger.tsx");

assertFileContains(projectPage, "ScheduleForm");
assertFileContains(projectPage, "ScheduleList");
assertFileContains(projectPage, "Расписание генерации");
assertFileContains(projectPage, "Расписание ссылок");
assertFileContains(scheduleForm, "Сейчас:");
assertFileContains(scheduleList, "ScheduleTrigger");
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
