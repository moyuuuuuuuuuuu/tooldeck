import fs from "node:fs";
import { Application } from "./src/application.js";

try {
  const input = JSON.parse(fs.readFileSync(0, "utf8"));
  process.stdout.write(JSON.stringify(new Application().run(input)));
} catch (error) {
  process.stderr.write(error instanceof Error ? error.message : String(error));
  process.exitCode = 1;
}
