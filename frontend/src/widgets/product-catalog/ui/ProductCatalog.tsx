import { JoinQueueProductCard } from '@features/join-queue';
import { ProductList } from '@features/product-list';

type Props = {
  excludeId?: string;
};

export const ProductCatalog = ({ excludeId }: Props): React.JSX.Element => {
  return (
    <section>
      <ProductList
        excludeId={excludeId}
        renderItem={(product) => <JoinQueueProductCard key={product.id} product={product} />}
      />
    </section>
  );
};
