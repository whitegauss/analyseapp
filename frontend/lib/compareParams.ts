/**
 * Parsing for the `?ids=` query the comparison page is linked with.
 *
 * The list is user-editable via the URL, so blanks and stray whitespace from a
 * hand-edited link have to survive parsing rather than becoming empty ids that
 * would each cost a doomed API call.
 */
export function parseCompareIds(idsParam: string | undefined): string[] {
  return (idsParam ?? "")
    .split(",")
    .map((id) => id.trim())
    .filter(Boolean);
}
