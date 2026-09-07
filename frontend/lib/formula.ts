// A small expression language for the error-propagation tool: parses a
// formula typed by hand (`2*pi*sqrt(L/g)`) and evaluates it together with its
// partial derivatives with respect to every variable in it.
//
// The derivatives are what error propagation needs, and they are computed by
// forward-mode automatic differentiation -- every node returns its value plus
// the gradient accumulated so far -- rather than by a finite difference. A
// numerical difference would need a step size per variable, and picking one
// badly is silently wrong (too large biases the slope, too small is eaten by
// floating-point cancellation). The chain rule applied node by node has no
// such knob and is exact to machine precision.

export class FormulaError extends Error {
  // Character offset the problem was found at, for pointing the user at it.
  readonly position: number;

  constructor(message: string, position: number) {
    super(message);
    this.name = "FormulaError";
    this.position = position;
  }
}

export type Node =
  | { type: "number"; value: number }
  | { type: "variable"; name: string }
  | { type: "constant"; name: string; value: number }
  | { type: "negate"; operand: Node }
  | { type: "binary"; op: "+" | "-" | "*" | "/" | "^"; left: Node; right: Node }
  | { type: "call"; name: string; args: Node[] };

export type ParsedFormula = {
  ast: Node;
  // Variable names in order of first appearance, so the input rows the UI
  // builds from this follow the formula as it reads.
  variables: string[];
};

// Values the parser resolves itself. A name in here is never a variable, so
// `pi` and `e` cannot be given a value or an uncertainty of their own.
const CONSTANTS: Record<string, number> = {
  pi: Math.PI,
  e: Math.E,
};

// Each function carries its own derivative. `derivatives[i]` is ∂f/∂(arg i)
// evaluated at the same arguments, which is all the chain rule needs.
type FunctionSpec = {
  arity: number;
  evaluate: (args: number[]) => number;
  derivatives: ((args: number[]) => number)[];
};

const FUNCTIONS: Record<string, FunctionSpec> = {
  sqrt: {
    arity: 1,
    evaluate: ([x]) => Math.sqrt(x),
    derivatives: [([x]) => 1 / (2 * Math.sqrt(x))],
  },
  exp: {
    arity: 1,
    evaluate: ([x]) => Math.exp(x),
    derivatives: [([x]) => Math.exp(x)],
  },
  ln: {
    arity: 1,
    evaluate: ([x]) => Math.log(x),
    derivatives: [([x]) => 1 / x],
  },
  log10: {
    arity: 1,
    evaluate: ([x]) => Math.log10(x),
    derivatives: [([x]) => 1 / (x * Math.LN10)],
  },
  sin: {
    arity: 1,
    evaluate: ([x]) => Math.sin(x),
    derivatives: [([x]) => Math.cos(x)],
  },
  cos: {
    arity: 1,
    evaluate: ([x]) => Math.cos(x),
    derivatives: [([x]) => -Math.sin(x)],
  },
  tan: {
    arity: 1,
    evaluate: ([x]) => Math.tan(x),
    derivatives: [([x]) => 1 / Math.cos(x) ** 2],
  },
  asin: {
    arity: 1,
    evaluate: ([x]) => Math.asin(x),
    derivatives: [([x]) => 1 / Math.sqrt(1 - x ** 2)],
  },
  acos: {
    arity: 1,
    evaluate: ([x]) => Math.acos(x),
    derivatives: [([x]) => -1 / Math.sqrt(1 - x ** 2)],
  },
  atan: {
    arity: 1,
    evaluate: ([x]) => Math.atan(x),
    derivatives: [([x]) => 1 / (1 + x ** 2)],
  },
  atan2: {
    arity: 2,
    evaluate: ([y, x]) => Math.atan2(y, x),
    derivatives: [
      ([y, x]) => x / (x ** 2 + y ** 2),
      ([y, x]) => -y / (x ** 2 + y ** 2),
    ],
  },
  sinh: {
    arity: 1,
    evaluate: ([x]) => Math.sinh(x),
    derivatives: [([x]) => Math.cosh(x)],
  },
  cosh: {
    arity: 1,
    evaluate: ([x]) => Math.cosh(x),
    derivatives: [([x]) => Math.sinh(x)],
  },
  tanh: {
    arity: 1,
    evaluate: ([x]) => Math.tanh(x),
    derivatives: [([x]) => 1 / Math.cosh(x) ** 2],
  },
  abs: {
    arity: 1,
    evaluate: ([x]) => Math.abs(x),
    derivatives: [([x]) => Math.sign(x)],
  },
};

// `log` is deliberately not a function: it means log10 in most lab handbooks
// and the natural log in most programming languages, and silently picking a
// base is worse than asking which one was meant.
const AMBIGUOUS_LOG =
  "log は底が曖昧です。ln（自然対数）か log10 と書いてください";

export const functionNames = Object.keys(FUNCTIONS);
export const constantNames = Object.keys(CONSTANTS);

type Token = {
  kind: "number" | "name" | "operator" | "paren" | "comma";
  text: string;
  position: number;
  value?: number;
};

const OPERATORS = new Set(["+", "-", "*", "/", "^"]);

function tokenize(source: string): Token[] {
  const tokens: Token[] = [];
  let i = 0;

  while (i < source.length) {
    const char = source[i];

    if (/\s/.test(char)) {
      i += 1;
      continue;
    }

    if (/[0-9.]/.test(char)) {
      const start = i;
      while (i < source.length && /[0-9.]/.test(source[i])) i += 1;
      // Exponent notation, but only when digits actually follow: `1.6e-19`
      // is one number, while the `e` in `2*e` is Euler's constant and the
      // one in `2e` is a typo the parser should complain about rather than
      // swallow.
      const exponentSign = /[+-]/.test(source[i + 1] ?? "") ? 1 : 0;
      if (
        /[eE]/.test(source[i] ?? "") &&
        /[0-9]/.test(source[i + 1 + exponentSign] ?? "")
      ) {
        i += 1 + exponentSign;
        while (i < source.length && /[0-9]/.test(source[i])) i += 1;
      }
      const text = source.slice(start, i);
      const value = Number(text);
      if (!Number.isFinite(value)) {
        throw new FormulaError(`数値として読めません: ${text}`, start);
      }
      tokens.push({ kind: "number", text, position: start, value });
      continue;
    }

    // Unicode letters, so Greek names physicists actually write (θ, λ, Δx)
    // are variables like any other.
    if (/[\p{L}_]/u.test(char)) {
      const start = i;
      while (i < source.length && /[\p{L}\p{N}_]/u.test(source[i])) i += 1;
      tokens.push({
        kind: "name",
        text: source.slice(start, i),
        position: start,
      });
      continue;
    }

    if (OPERATORS.has(char)) {
      tokens.push({ kind: "operator", text: char, position: i });
      i += 1;
      continue;
    }

    if (char === "(" || char === ")") {
      tokens.push({ kind: "paren", text: char, position: i });
      i += 1;
      continue;
    }

    if (char === ",") {
      tokens.push({ kind: "comma", text: char, position: i });
      i += 1;
      continue;
    }

    throw new FormulaError(`使えない文字です: ${char}`, i);
  }

  return tokens;
}

// Recursive descent over the usual arithmetic precedence:
//
//   expression := term (('+' | '-') term)*
//   term       := unary (('*' | '/') unary)*
//   unary      := ('-' | '+') unary | power
//   power      := atom ('^' unary)?
//   atom       := number | name | name '(' args ')' | '(' expression ')'
//
// `^` is right-associative (a^b^c = a^(b^c)) and binds tighter than unary
// minus (-x^2 = -(x^2)), matching how the same formula reads on paper. Its
// right side is a unary so `x^-2` works.
class Parser {
  private index = 0;

  constructor(
    private readonly tokens: Token[],
    private readonly source: string,
    readonly variables: string[] = [],
  ) {}

  parse(): Node {
    if (this.tokens.length === 0) {
      throw new FormulaError("式が空です", 0);
    }
    const node = this.parseExpression();
    const leftover = this.peek();
    if (leftover) {
      throw new FormulaError(
        `式の続きが読めません: ${leftover.text}`,
        leftover.position,
      );
    }
    return node;
  }

  private peek(): Token | undefined {
    return this.tokens[this.index];
  }

  private next(): Token | undefined {
    return this.tokens[this.index++];
  }

  private endPosition(): number {
    return this.source.length;
  }

  private parseExpression(): Node {
    let left = this.parseTerm();
    for (;;) {
      const token = this.peek();
      if (
        token?.kind === "operator" &&
        (token.text === "+" || token.text === "-")
      ) {
        this.index += 1;
        left = {
          type: "binary",
          op: token.text,
          left,
          right: this.parseTerm(),
        };
        continue;
      }
      return left;
    }
  }

  private parseTerm(): Node {
    let left = this.parseUnary();
    for (;;) {
      const token = this.peek();
      if (
        token?.kind === "operator" &&
        (token.text === "*" || token.text === "/")
      ) {
        this.index += 1;
        left = {
          type: "binary",
          op: token.text,
          left,
          right: this.parseUnary(),
        };
        continue;
      }
      return left;
    }
  }

  private parseUnary(): Node {
    const token = this.peek();
    if (
      token?.kind === "operator" &&
      (token.text === "-" || token.text === "+")
    ) {
      this.index += 1;
      const operand = this.parseUnary();
      return token.text === "-" ? { type: "negate", operand } : operand;
    }
    return this.parsePower();
  }

  private parsePower(): Node {
    const base = this.parseAtom();
    const token = this.peek();
    if (token?.kind === "operator" && token.text === "^") {
      this.index += 1;
      return { type: "binary", op: "^", left: base, right: this.parseUnary() };
    }
    return base;
  }

  private parseAtom(): Node {
    const token = this.next();
    if (!token) {
      throw new FormulaError("式が途中で終わっています", this.endPosition());
    }

    if (token.kind === "number") {
      return { type: "number", value: token.value as number };
    }

    if (token.kind === "paren" && token.text === "(") {
      const inner = this.parseExpression();
      const closing = this.next();
      if (!closing || closing.kind !== "paren" || closing.text !== ")") {
        throw new FormulaError(
          "閉じ括弧が足りません",
          closing?.position ?? this.endPosition(),
        );
      }
      return inner;
    }

    if (token.kind === "name") {
      const following = this.peek();
      const isCall = following?.kind === "paren" && following.text === "(";

      if (isCall) {
        return this.parseCall(token);
      }

      if (Object.hasOwn(CONSTANTS, token.text)) {
        return {
          type: "constant",
          name: token.text,
          value: CONSTANTS[token.text],
        };
      }
      if (Object.hasOwn(FUNCTIONS, token.text)) {
        throw new FormulaError(
          `${token.text} は関数です。${token.text}(...) のように括弧が要ります`,
          token.position,
        );
      }
      if (token.text === "log") {
        throw new FormulaError(AMBIGUOUS_LOG, token.position);
      }
      if (!this.variables.includes(token.text)) {
        this.variables.push(token.text);
      }
      return { type: "variable", name: token.text };
    }

    throw new FormulaError(`ここには置けません: ${token.text}`, token.position);
  }

  private parseCall(name: Token): Node {
    const spec = Object.hasOwn(FUNCTIONS, name.text)
      ? FUNCTIONS[name.text]
      : undefined;
    if (!spec && name.text === "log") {
      throw new FormulaError(AMBIGUOUS_LOG, name.position);
    }
    if (!spec) {
      throw new FormulaError(
        `知らない関数です: ${name.text}（使えるのは ${functionNames.join(", ")}）`,
        name.position,
      );
    }

    this.index += 1; // consume "("
    const args: Node[] = [];
    if (this.peek()?.kind === "paren" && this.peek()?.text === ")") {
      this.index += 1;
    } else {
      for (;;) {
        args.push(this.parseExpression());
        const separator = this.next();
        if (separator?.kind === "comma") continue;
        if (separator?.kind === "paren" && separator.text === ")") break;
        throw new FormulaError(
          "引数の区切り（,）か閉じ括弧がありません",
          separator?.position ?? this.endPosition(),
        );
      }
    }

    if (args.length !== spec.arity) {
      throw new FormulaError(
        `${name.text} は引数が ${spec.arity} 個です（${args.length} 個ありました）`,
        name.position,
      );
    }
    return { type: "call", name: name.text, args };
  }
}

export function parseFormula(source: string): ParsedFormula {
  const parser = new Parser(tokenize(source), source);
  const ast = parser.parse();
  return { ast, variables: parser.variables };
}

export type Evaluation = {
  value: number;
  // ∂(formula)/∂name for every variable in the formula.
  gradient: Record<string, number>;
};

// Forward-mode automatic differentiation: each node reports its value and the
// partial derivative of that value with respect to every variable seen so
// far. Combining two nodes applies the ordinary product/quotient/chain rules
// to both halves at once.
export function evaluateWithGradient(
  ast: Node,
  values: Record<string, number>,
): Evaluation {
  return visit(ast, values);
}

function visit(node: Node, values: Record<string, number>): Evaluation {
  switch (node.type) {
    case "number":
      return { value: node.value, gradient: emptyGradient() };

    case "constant":
      return { value: node.value, gradient: emptyGradient() };

    case "variable": {
      const value = Object.hasOwn(values, node.name)
        ? values[node.name]
        : undefined;
      if (value === undefined) {
        throw new FormulaError(`${node.name} の値がありません`, 0);
      }
      // ∂x/∂x = 1; every other variable's partial stays 0 by omission.
      const gradient = emptyGradient();
      gradient[node.name] = 1;
      return { value, gradient };
    }

    case "negate": {
      const operand = visit(node.operand, values);
      return {
        value: -operand.value,
        gradient: scale(operand.gradient, -1),
      };
    }

    case "binary":
      return visitBinary(node, values);

    case "call": {
      const spec = FUNCTIONS[node.name];
      const args = node.args.map((arg) => visit(arg, values));
      const argValues = args.map((arg) => arg.value);
      let gradient = emptyGradient();
      args.forEach((arg, i) => {
        gradient = add(
          gradient,
          scale(arg.gradient, spec.derivatives[i](argValues)),
        );
      });
      return { value: spec.evaluate(argValues), gradient };
    }
  }
}

function visitBinary(
  node: Extract<Node, { type: "binary" }>,
  values: Record<string, number>,
): Evaluation {
  const left = visit(node.left, values);
  const right = visit(node.right, values);

  switch (node.op) {
    case "+":
      return {
        value: left.value + right.value,
        gradient: add(left.gradient, right.gradient),
      };
    case "-":
      return {
        value: left.value - right.value,
        gradient: add(left.gradient, scale(right.gradient, -1)),
      };
    case "*":
      return {
        value: left.value * right.value,
        gradient: add(
          scale(left.gradient, right.value),
          scale(right.gradient, left.value),
        ),
      };
    case "/":
      return {
        value: left.value / right.value,
        gradient: add(
          scale(left.gradient, 1 / right.value),
          scale(right.gradient, -left.value / right.value ** 2),
        ),
      };
    case "^": {
      const value = Math.pow(left.value, right.value);
      // d/dx x^n = n·x^(n-1) for the base, and d/dn x^n = x^n·ln x for the
      // exponent. The second term only exists when the exponent itself
      // depends on a variable (`x^n` with n measured); for a constant
      // exponent its gradient is empty and the ln disappears with it, which
      // matters because ln x is not defined for a negative base.
      let gradient = scale(
        left.gradient,
        right.value * Math.pow(left.value, right.value - 1),
      );
      if (Object.keys(right.gradient).length > 0) {
        gradient = add(
          gradient,
          scale(right.gradient, value * Math.log(left.value)),
        );
      }
      return { value, gradient };
    }
  }
}

// A gradient is keyed by variable names the user typed, so it is built
// without a prototype: `constructor` must not resolve to something inherited,
// and `__proto__ = 1` on a plain object silently sets the prototype instead
// of storing the partial, which would drop that variable from the result.
function emptyGradient(): Record<string, number> {
  return Object.create(null) as Record<string, number>;
}

function scale(
  gradient: Record<string, number>,
  factor: number,
): Record<string, number> {
  const scaled = emptyGradient();
  for (const [name, partial] of Object.entries(gradient)) {
    scaled[name] = partial * factor;
  }
  return scaled;
}

function add(
  a: Record<string, number>,
  b: Record<string, number>,
): Record<string, number> {
  const sum = emptyGradient();
  Object.assign(sum, a);
  for (const [name, partial] of Object.entries(b)) {
    sum[name] = (sum[name] ?? 0) + partial;
  }
  return sum;
}
