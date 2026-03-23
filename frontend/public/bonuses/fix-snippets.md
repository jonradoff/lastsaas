# Shopify MCP Fix Snippets — Agency Tier Bonus
## 10 Most Common Issues with Exact Fixes

> Copy-paste ready. Tested on Shopify 2.0 themes and Storefront API v2024-01.
> All Liquid snippets assume Online Store 2.0 theme architecture.

---

## Fix 1: Missing Product Prices

**Problem:** Product prices are not populated or are returning null/zero via the MCP endpoint.

**Why it matters for AI agents:** Agents cannot recommend, rank, or compare products without accurate price data — they skip or de-rank zero-price products entirely.

**How to fix:**

1. In your Shopify Admin, go to **Products** and bulk-filter for products with no price set.
2. In your theme's `product.liquid` or product JSON template, ensure price is rendered:

```liquid
{% if product.selected_or_first_available_variant.price %}
  <span class="price" data-price="{{ product.selected_or_first_available_variant.price | money_without_currency }}">
    {{ product.selected_or_first_available_variant.price | money }}
  </span>
{% else %}
  <span class="price price--unavailable">Price not available</span>
{% endif %}
```

3. For the Storefront API / MCP endpoint, verify that `priceRange` is included in your product query:

```graphql
query GetProduct($handle: String!) {
  product(handle: $handle) {
    title
    priceRange {
      minVariantPrice {
        amount
        currencyCode
      }
      maxVariantPrice {
        amount
        currencyCode
      }
    }
  }
}
```

4. If prices are missing only for specific variants, check that variants have a price set in the Shopify Admin under **Products > [Product] > Variants**.

**Estimated fix time:** 1–2 hours (audit + bulk update via CSV if needed)

---

## Fix 2: Short Product Descriptions

**Problem:** Product descriptions are too brief (under 80 words) for AI agents to extract meaningful information.

**Why it matters for AI agents:** Agents use description content to match products to buyer intent. Short descriptions reduce match confidence and lead agents to recommend competitors with richer content.

**How to fix:**

Use this template as a starting structure for expanding product descriptions:

```
[Product Name] is a [category] designed for [target use case/customer].

Key specifications:
- Material: [material]
- Dimensions: [L x W x H] / Weight: [weight]
- [Spec 3]: [value]
- [Spec 4]: [value]
- Compatibility: [compatible systems/products]

[2–3 sentences about what problems it solves or benefits it delivers]

Ideal for: [use case 1], [use case 2], [use case 3]

Includes: [what's in the box]

[Optional: care instructions, warranty, certifications]
```

For bulk updates, use Shopify's CSV product export, expand descriptions in a spreadsheet, and re-import. Alternatively, use the Admin API:

```javascript
// Shopify Admin API — update product description
await fetch(`/admin/api/2024-01/products/${productId}.json`, {
  method: 'PUT',
  headers: { 'Content-Type': 'application/json', 'X-Shopify-Access-Token': token },
  body: JSON.stringify({
    product: {
      id: productId,
      body_html: '<p>Your expanded description here...</p>'
    }
  })
});
```

**Estimated fix time:** 30–60 minutes per product (manual); 1 week for AI-assisted bulk rewrite of 100 products

---

## Fix 3: Missing Structured Attributes (Metafields)

**Problem:** Products lack structured data in metafields, so attributes like material, dimensions, and certifications are buried in description text or missing entirely.

**Why it matters for AI agents:** Agents parse structured fields first. Unstructured text in descriptions is less reliable for attribute extraction, especially for filtering and comparison tasks.

**How to fix:**

1. In Shopify Admin, go to **Settings > Custom Data > Products** and create metafield definitions:

```
Namespace: custom
Key: material
Type: Single line text

Namespace: custom
Key: dimensions
Type: Single line text (e.g., "30cm x 20cm x 10cm")

Namespace: custom
Key: weight_grams
Type: Integer

Namespace: custom
Key: certifications
Type: List of single line text
```

2. Expose metafields in your theme's product template:

```liquid
{% if product.metafields.custom.material %}
  <div class="product-metafield" data-attribute="material">
    <span class="label">Material:</span>
    <span class="value">{{ product.metafields.custom.material }}</span>
  </div>
{% endif %}
```

3. Query metafields via Storefront API (requires explicit access grant):

```graphql
query GetProductWithMetafields($handle: String!) {
  product(handle: $handle) {
    title
    metafields(identifiers: [
      { namespace: "custom", key: "material" },
      { namespace: "custom", key: "dimensions" }
    ]) {
      key
      value
      type
    }
  }
}
```

**Estimated fix time:** 2–4 hours to set up definitions + 1–2 days to populate for existing catalog

---

## Fix 4: Invalid or Broken Image URLs

**Problem:** Product images return 404, are hosted on third-party CDNs that have expired, or are missing alt text.

**Why it matters for AI agents:** Agents with vision capabilities cannot process broken images. Missing alt text degrades product context and search relevance.

**How to fix:**

**Checklist for image URL validation:**
- [ ] All images load directly from `cdn.shopify.com` (not external CDNs)
- [ ] No images return HTTP 404 or 403
- [ ] All images have descriptive `alt` text (not empty or filename)
- [ ] Featured image is set for every product

In your theme, ensure alt text is populated:

```liquid
<img
  src="{{ product.featured_image | image_url: width: 800 }}"
  alt="{{ product.featured_image.alt | default: product.title | escape }}"
  width="800"
  height="{{ 800 | divided_by: product.featured_image.aspect_ratio | round }}"
  loading="lazy"
/>
```

To audit broken images in bulk, use the Shopify Admin API to list all product images and check their URLs:

```javascript
// Check image availability
const products = await fetch('/admin/api/2024-01/products.json?fields=id,title,images');
const data = await products.json();
for (const product of data.products) {
  for (const img of product.images) {
    const res = await fetch(img.src, { method: 'HEAD' });
    if (!res.ok) console.log(`Broken image: ${product.title} — ${img.src}`);
  }
}
```

**Estimated fix time:** 30 minutes to audit + 1–4 hours to fix depending on catalog size

---

## Fix 5: Missing Product Variants

**Problem:** Products that come in multiple sizes, colors, or configurations have variants missing or not properly configured.

**Why it matters for AI agents:** Agents match buyer intent to specific variants (e.g., "blue, size M"). If variants aren't configured, agents cannot fulfill specific requests and may drop the recommendation.

**How to fix:**

**Variant setup best practices:**

1. Every product option dimension (size, color, material) should be a Shopify variant — not free-form text in the description.
2. Each variant should have: a unique SKU, a price (can be same as parent), an inventory quantity, and optionally a variant-specific image.

```liquid
<!-- Render variant selector in theme -->
<select name="id" id="variant-selector">
  {% for variant in product.variants %}
    <option
      value="{{ variant.id }}"
      {% if variant == product.selected_or_first_available_variant %}selected{% endif %}
      {% unless variant.available %}disabled{% endunless %}
      data-sku="{{ variant.sku }}"
      data-price="{{ variant.price }}"
    >
      {{ variant.title }}{% unless variant.available %} — Sold Out{% endunless %}
    </option>
  {% endfor %}
</select>
```

3. Verify variants are surfaced in the Storefront API:

```graphql
query {
  product(handle: "your-product") {
    variants(first: 50) {
      edges {
        node {
          id
          title
          sku
          price { amount currencyCode }
          availableForSale
          selectedOptions { name value }
        }
      }
    }
  }
}
```

**Estimated fix time:** 1–2 hours per product (manual variant setup); use bulk operations for catalog-wide changes

---

## Fix 6: Slow MCP Response Times

**Problem:** The Shopify MCP endpoint takes more than 2 seconds to respond to agent queries.

**Why it matters for AI agents:** Agents have timeout budgets. Slow stores are skipped or deprioritized in favor of faster responses. A 2s+ response time significantly reduces agent engagement.

**Common causes and fixes:**

| Cause | Fix |
|-------|-----|
| Large product catalog with no pagination | Implement cursor-based pagination in Storefront API queries |
| Unoptimized Storefront API query fetching all fields | Use field selection — only request fields agents actually need |
| No CDN caching on product API responses | Enable Shopify Storefront API caching headers |
| Heavy theme JavaScript blocking page render | Not directly MCP-related; focus on API query optimization |
| Images served at full resolution | Use Shopify's image transformation API (`image_url: width: 400`) |

**Optimized Storefront API query (minimal fields for agent use):**

```graphql
query AgentProductQuery($handle: String!) {
  product(handle: $handle) {
    id
    title
    description
    priceRange {
      minVariantPrice { amount currencyCode }
    }
    featuredImage { url altText }
    variants(first: 10) {
      edges {
        node {
          id
          title
          price { amount currencyCode }
          availableForSale
          selectedOptions { name value }
        }
      }
    }
  }
}
```

**Estimated fix time:** 2–4 hours for query optimization; ongoing monitoring recommended

---

## Fix 7: Missing Product Categories (Taxonomy)

**Problem:** Products are not assigned to Shopify's standard product taxonomy categories, or use inconsistent custom tags.

**Why it matters for AI agents:** Agents use category data for filtering and search. Miscategorized or uncategorized products are harder to surface in category-level queries (e.g., "show me running shoes under $100").

**How to fix:**

1. In Shopify Admin, go to each product and set the **Category** field using Shopify's standard product taxonomy (available since Shopify 2023).

2. Use the Admin API to bulk-assign categories:

```javascript
await fetch(`/admin/api/2024-01/products/${productId}.json`, {
  method: 'PUT',
  headers: { 'Content-Type': 'application/json', 'X-Shopify-Access-Token': token },
  body: JSON.stringify({
    product: {
      id: productId,
      product_type: 'Running Shoes', // Maps to Shopify taxonomy
      tags: 'running, athletic, footwear, mens' // Supplemental tags
    }
  })
});
```

3. Ensure collections have clear, descriptive names and are set to automated rules where possible.

4. Verify taxonomy is accessible via Storefront API:

```graphql
query {
  product(handle: "product-handle") {
    productType
    tags
    collections(first: 5) {
      edges { node { title handle } }
    }
  }
}
```

**Estimated fix time:** 1–3 days for catalog-wide taxonomy audit and assignment

---

## Fix 8: No Checkout Capability for AI Agents

**Problem:** The store's MCP/Storefront API is not configured to allow agents to add items to cart and initiate checkout.

**Why it matters for AI agents:** Read-only stores cannot complete agent-driven purchases. Agents that can browse but not buy have zero conversion value.

**How to fix:**

1. Ensure your Shopify Storefront API token has the `unauthenticated_write_checkouts` and `unauthenticated_write_customers` scopes.

2. Implement cart creation via Storefront API:

```graphql
mutation CreateCart($lines: [CartLineInput!]!) {
  cartCreate(input: { lines: $lines }) {
    cart {
      id
      checkoutUrl
      lines(first: 10) {
        edges {
          node {
            id
            quantity
            merchandise {
              ... on ProductVariant {
                id
                title
                price { amount currencyCode }
              }
            }
          }
        }
      }
    }
    userErrors { field message }
  }
}
```

3. The `checkoutUrl` returned can be passed to the customer to complete purchase, or used by agents in autonomous checkout flows.

4. Test the full flow: create cart → add line items → retrieve checkout URL → verify it opens correctly.

**Estimated fix time:** 4–8 hours for full implementation and testing

---

## Fix 9: Missing Inventory Data

**Problem:** Product inventory quantities are not tracked or not exposed via the API, so agents cannot determine availability.

**Why it matters for AI agents:** Agents avoid recommending out-of-stock products. Without inventory data, agents either assume availability (risking bad recommendations) or skip the product entirely.

**How to fix:**

1. In Shopify Admin, enable inventory tracking for all products: **Products > [Product] > Inventory > Track quantity**.

2. Set inventory levels accurately for each location.

3. Verify inventory is exposed via Storefront API:

```graphql
query {
  product(handle: "product-handle") {
    variants(first: 10) {
      edges {
        node {
          id
          title
          availableForSale
          quantityAvailable
          currentlyNotInStock
        }
      }
    }
  }
}
```

4. In your Liquid theme, expose inventory data for agent-readable page rendering:

```liquid
{% for variant in product.variants %}
  <div
    data-variant-id="{{ variant.id }}"
    data-available="{{ variant.available }}"
    data-inventory="{{ variant.inventory_quantity }}"
  >
    {{ variant.title }}:
    {% if variant.available %}
      In Stock ({{ variant.inventory_quantity }} left)
    {% else %}
      Out of Stock
    {% endif %}
  </div>
{% endfor %}
```

**Estimated fix time:** 1–2 hours to enable tracking + time to audit/update actual inventory counts

---

## Fix 10: Poor Search Relevance (Product Tagging & Description Optimization)

**Problem:** Products are not surfaced when agents search by intent, synonym, or use case — because tags and descriptions don't match how buyers describe what they want.

**Why it matters for AI agents:** Agents translate natural language queries into search terms. If your product tags and descriptions don't include buyer vocabulary, intent-matching fails and the agent recommends a competitor.

**How to fix:**

**Product tagging strategy:**

```
Tags should include:
- Primary category (e.g., "running-shoes")
- Use cases (e.g., "trail-running", "marathon", "everyday-training")
- Key attributes (e.g., "waterproof", "lightweight", "wide-fit")
- Buyer intent terms (e.g., "best-for-beginners", "high-arch-support")
- Synonyms (e.g., "trainers", "sneakers", "athletic-shoes")
```

In Liquid, ensure tags are rendered in a machine-readable format:

```liquid
<div class="product-tags" aria-hidden="true" style="display:none;">
  {% for tag in product.tags %}
    <span class="tag">{{ tag }}</span>
  {% endfor %}
</div>
```

**Description optimization checklist:**
- [ ] Include the full product name in the first sentence
- [ ] Use the 3–5 most common buyer search terms naturally in the description
- [ ] Include "ideal for" or "designed for" phrases that match use cases
- [ ] List materials and key specs as bullet points (not prose)
- [ ] Include comparison anchors where relevant ("lighter than [competitor product]")

**Estimated fix time:** 2–4 hours for tagging strategy + 20–30 minutes per product to optimize descriptions; use AI writing tools to scale
