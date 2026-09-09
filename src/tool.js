export class Tool {
  run(input) {
    throw new Error("run() must be implemented");
  }

  cancel(input, state) {
    throw new Error("cancel() must be implemented");
  }
}
