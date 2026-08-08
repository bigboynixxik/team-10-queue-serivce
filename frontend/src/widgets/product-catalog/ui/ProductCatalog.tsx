import { JoinQueueButton } from '@features/join-queue';
import { ProductList } from '@features/product-list';
import { Typography } from '@ui';

export const ProductCatalog = (): React.JSX.Element => {
  return (
    <section>
      <Typography.Title level={2}>Товары с ограниченным остатком</Typography.Title>
      <ProductList renderAction={(product) => <JoinQueueButton product={product} />} />
    </section>
  );
};
