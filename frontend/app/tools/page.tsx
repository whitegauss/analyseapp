import CenteredCard from "@/components/CenteredCard";
import ToolsCalculator from "@/components/tools/ToolsCalculator";

export const metadata = {
  title: "計算ツール | AnalyseApp",
};

// A standalone utility page, independent of any experiment or login state
// (unlike /experiments/*, which is always scoped to a saved experiment).
export default function ToolsPage() {
  return (
    <CenteredCard maxWidth="max-w-3xl">
      <h1 className="text-2xl font-semibold text-zinc-900 dark:text-zinc-50">
        計算ツール
      </h1>
      <ToolsCalculator />
    </CenteredCard>
  );
}
