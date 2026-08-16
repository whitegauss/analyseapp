import type { ReactNode } from "react";

type Props = {
  maxWidth: string;
  outerPadded?: boolean;
  verticallyCentered?: boolean;
  children: ReactNode;
};

// The "centered card" shell shared by every top-level page (login, signup,
// top page, experiments list, experiment detail) -- only the card's max
// width and a couple of outer-spacing details differ. `flex-1` (rather than
// its own min-h-screen) fills the space RootLayout leaves between the
// persistent Header and Footer.
export default function CenteredCard({
  maxWidth,
  outerPadded = true,
  verticallyCentered = true,
  children,
}: Props) {
  const outerClasses = [
    "flex flex-1 justify-center bg-zinc-50 dark:bg-black",
    verticallyCentered && "items-center",
    outerPadded && "p-8",
  ]
    .filter(Boolean)
    .join(" ");

  return (
    <div className={outerClasses}>
      <main
        className={`flex w-full ${maxWidth} flex-col gap-6 rounded-lg border border-zinc-200 bg-white p-8 dark:border-zinc-800 dark:bg-zinc-900`}
      >
        {children}
      </main>
    </div>
  );
}
