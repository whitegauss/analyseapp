import CenteredCard from "@/components/CenteredCard";
import ToolsCalculator from "@/components/tools/ToolsCalculator";

export const metadata = {
  title: "計算ツール | AnalyseApp",
};

// A standalone utility page, independent of any experiment or login state
// (unlike /experiments/*, which is always scoped to a saved experiment).
//
// Not vertically centered: the four tools differ a lot in height, and a
// centered card grows around its middle, so switching tabs would slide the
// heading and the tab row up or down under the pointer. Anchored to the top,
// only the bottom edge moves.
export default function ToolsPage() {
  return (
    <CenteredCard maxWidth="max-w-3xl" verticallyCentered={false}>
      <h1 className="text-2xl font-semibold text-zinc-900 dark:text-zinc-50">
        計算ツール
      </h1>
      <ToolsCalculator />
    </CenteredCard>
  );
}
