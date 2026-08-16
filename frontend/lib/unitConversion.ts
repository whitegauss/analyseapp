export type Unit = {
  id: string;
  label: string;
  toBase: (v: number) => number;
  fromBase: (v: number) => number;
};

export type UnitCategory = {
  id: string;
  label: string;
  units: Unit[];
};

// A pure scale-factor unit (base = value * factor) -- covers everything
// except temperature, which needs an offset too.
function linear(id: string, label: string, factor: number): Unit {
  return { id, label, toBase: (v) => v * factor, fromBase: (v) => v / factor };
}

export const unitCategories: UnitCategory[] = [
  {
    id: "length",
    label: "長さ",
    units: [
      linear("m", "m", 1),
      linear("cm", "cm", 0.01),
      linear("mm", "mm", 0.001),
      linear("km", "km", 1000),
      linear("in", "in（インチ）", 0.0254),
      linear("ft", "ft（フィート）", 0.3048),
    ],
  },
  {
    id: "mass",
    label: "質量",
    units: [
      linear("kg", "kg", 1),
      linear("g", "g", 0.001),
      linear("mg", "mg", 0.000001),
      linear("lb", "lb（ポンド）", 0.45359237),
    ],
  },
  {
    id: "time",
    label: "時間",
    units: [
      linear("s", "s", 1),
      linear("min", "min", 60),
      linear("h", "h", 3600),
    ],
  },
  {
    id: "angle",
    label: "角度",
    units: [linear("rad", "rad", 1), linear("deg", "deg（°）", Math.PI / 180)],
  },
  {
    id: "temperature",
    label: "温度",
    units: [
      { id: "K", label: "K", toBase: (v) => v, fromBase: (v) => v },
      {
        id: "C",
        label: "°C",
        toBase: (v) => v + 273.15,
        fromBase: (v) => v - 273.15,
      },
      {
        id: "F",
        label: "°F",
        toBase: (v) => ((v - 32) * 5) / 9 + 273.15,
        fromBase: (v) => ((v - 273.15) * 9) / 5 + 32,
      },
    ],
  },
];

export function convertUnit(
  categoryId: string,
  fromUnitId: string,
  toUnitId: string,
  value: number,
): number | null {
  const category = unitCategories.find((c) => c.id === categoryId);
  const from = category?.units.find((u) => u.id === fromUnitId);
  const to = category?.units.find((u) => u.id === toUnitId);
  if (!from || !to) return null;
  return to.fromBase(from.toBase(value));
}
