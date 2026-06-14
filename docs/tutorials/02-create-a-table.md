# 2. Create a table

**Goal:** create a table and give it fields.
**Before you start:** you finished [tutorial 1](01-create-an-application.md).
**Time:** ~7 minutes.

A *table* in devengine is an **entity type** — a named place to keep records of
the same kind. A *field* is an **attribute**: each one has a type that decides
what it stores and how it shows up in forms. You define the table, add its
fields, then (next tutorial) build a form over it. We'll make a `Product` table
for our inventory.

## Create the table

1. In the left sidebar, open **Tools → Database Schema**.
2. Click **New EAV Table** (top right).
3. Fill in:
   - **Name** — `Product`
   - **Machine name** — leave it. It fills in as `product` while you type.
     This is the table's identifier in URLs and scripts; lowercase, no spaces.
4. Leave **Description** and the two Filo script boxes empty for now.
5. Click **Create table**.

You land on the new table's page. Under **Attributes** you'll see
*"No attributes yet."* — that's expected; let's add some.

## Add the first field

1. Under **Attributes**, click **New attribute**.
2. Fill in:
   - **Label** — `Name`
   - **Machine name** — fills in as `name`
   - **Type** — choose **TEXT - Text**
3. Check **Required field** — a product must have a name.
4. Click **Create attribute** (at the bottom).

You return to the `Product` page and `name` now appears under **Attributes**.

## Add the rest

Repeat **New attribute → fill in → Create attribute** for each row below. Only
`Name` is required; leave the others optional. Together they cover all five
field types.

| Label | Machine name | Type | Required |
|-------|--------------|------|----------|
| Name | `name` | TEXT - Text | yes (done above) |
| Quantity | `quantity` | INT - Integer number | no |
| Price | `price` | REAL - Decimal number | no |
| In stock | `in_stock` | BOOL - True/False | no |
| Received on | `received_on` | DATETIME - Date/Time | no |

## Check it worked

The `Product` page lists all five attributes with their types: `name` (TEXT,
required), `quantity` (INT), `price` (REAL), `in_stock` (BOOL) and
`received_on` (DATETIME).

## The five types

When you pick a **Type**, these are your choices:

| Type | Stores | Example |
|------|--------|---------|
| TEXT - Text | text | a name, a description |
| INT - Integer number | whole numbers | a quantity, a count |
| REAL - Decimal number | numbers with decimals | a price, a weight |
| BOOL - True/False | yes / no | in stock? |
| DATETIME - Date/Time | a date and time | received on |

The type is fixed once the attribute exists — pick it deliberately. devengine
stores records in a flexible entity-attribute-value layout, so you never write
SQL or run a migration to add a table or a field; you just define them here.

## Next

→ [3. Create a form and use it](03-create-a-form.md)
