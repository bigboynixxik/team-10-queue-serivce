export type CtaAction = {
  label: string;
  run: () => void;
  disabled?: boolean;
};

type ResolveCtaActionParams = {
  isPayable: boolean;
  isQueued: boolean;
  isSoldOut: boolean;
  pay: () => void;
  submit: () => void;
  goHome: () => void;
};

export const resolveCtaAction = ({
  isPayable,
  isQueued,
  isSoldOut,
  pay,
  submit,
  goHome,
}: ResolveCtaActionParams): CtaAction => {
  if (isPayable) return { label: 'Оплатить товар', run: pay };
  if (isQueued) return { label: 'Вы в очереди', run: () => {}, disabled: true };
  if (isSoldOut) return { label: 'Вернуться к товарам', run: goHome };

  return { label: 'Перейти в очередь', run: submit };
};
