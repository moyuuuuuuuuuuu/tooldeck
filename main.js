import fs from "node:fs";
import { Application } from "./src/application.js";

try {
  const input = JSON.parse(fs.readFileSync(0, "utf8"));
  const application = new Application();
  if (process.env.TOOLDECK_ACTION === "cancel") {
    const stateFile = process.env.TOOLDECK_STATE_FILE;
    const state = stateFile && fs.existsSync(stateFile)
      ? JSON.parse(fs.readFileSync(stateFile, "utf8"))
      : {};
    application.cancel(input, state);
  } else {
    process.stdout.write(JSON.stringify(application.run(input)));
  }
} catch (error) {
  process.stderr.write(error instanceof Error ? error.message : String(error));
  process.exitCode = 1;
}
