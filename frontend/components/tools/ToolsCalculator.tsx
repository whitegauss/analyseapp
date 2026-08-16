"use client";

import { useState } from "react";
import ErrorPropagationCalculator from "./ErrorPropagationCalculator";
import SignificantFigureRounder from "./SignificantFigureRounder";
import UnitConverter from "./UnitConverter";
import MeasurementStatsCalculator from "./MeasurementStatsCalculator";

const TABS = [
  { id: "propagation", label: "誤差伝播" },
  { id: "rounding", label: "有効数字の丸め" },
  { id: "unit", label: "単位変換" },
  { id: "stats", label: "統計量" },
] as const;

type TabId = (typeof TABS)[number]["id"];

export default function ToolsCalculator() {
  const [activeTab, setActiveTab] = useState<TabId>("propagation");

  return (
    <div className="flex flex-col gap-6">
      <div className="flex gap-2 border-b border-zinc-200 dark:border-zinc-800">
        {TABS.map((tab) => (
          <button
            key={tab.id}
            type="button"
            onClick={() => setActiveTab(tab.id)}
            className={`px-3 py-2 text-sm font-medium ${
              activeTab === tab.id
                ? "border-b-2 border-zinc-900 text-zinc-900 dark:border-zinc-50 dark:text-zinc-50"
                : "text-zinc-500 dark:text-zinc-400"
            }`}
          >
            {tab.label}
          </button>
        ))}
      </div>

      {activeTab === "propagation" && <ErrorPropagationCalculator />}
      {activeTab === "rounding" && <SignificantFigureRounder />}
      {activeTab === "unit" && <UnitConverter />}
      {activeTab === "stats" && <MeasurementStatsCalculator />}
    </div>
  );
}
