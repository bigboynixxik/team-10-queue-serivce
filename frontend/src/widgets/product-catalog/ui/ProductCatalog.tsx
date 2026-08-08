import { JoinQueueButton } from '@features/join-queue';
import { ProductList } from '@features/product-list';

export const ProductCatalog = (): React.JSX.Element => {
  return (
    <section>
      <ProductList renderAction={(product) => <JoinQueueButton product={product} />} />
    </section>
  );
};
