type Modifiers = Record<string, boolean | null | undefined>;

export const cn =
  (block: string) =>
  (element?: string, modifiers?: Modifiers): string => {
    const base = element ? `${block}__${element}` : block;
    const enabledModifiers = Object.entries(modifiers ?? {})
      .filter(([, enabled]) => enabled)
      .map(([modifier]) => `${base}--${modifier}`);

    return enabledModifiers.length ? enabledModifiers.join(' ') : base;
  };
