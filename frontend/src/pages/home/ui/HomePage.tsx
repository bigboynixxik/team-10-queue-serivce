import { Typography } from '@ui';
import { ProductCatalog } from '@widgets/product-catalog';

export function HomePage() {
  return (
    <main className="page-content">
      <section className="hero">
        <Typography.Title>Авито Очередь</Typography.Title>
        <Typography.Paragraph>
          Покупайте редкие товары честно: встаньте в очередь и получите время на оплату, когда товар
          станет доступен.
        </Typography.Paragraph>
      </section>
      <ProductCatalog />
    </main>
  );
}
