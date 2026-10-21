export default async function* (events) {
  for await (const event of events) {
    if (["test:pass", "test:fail", "test:summary"].includes(event.type))
      yield JSON.stringify(event) + "\n";
  }
}
