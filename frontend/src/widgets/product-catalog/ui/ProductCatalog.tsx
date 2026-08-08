import { JoinQueueProductCard } from '@features/join-queue';
import { ProductList } from '@features/product-list';

export const ProductCatalog = (): React.JSX.Element => {
  return (
    <section>
      <ProductList
        renderItem={(product) => <JoinQueueProductCard key={product.id} product={product} />}
      />
    </section>
  );
};
