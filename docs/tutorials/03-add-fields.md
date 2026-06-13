# 3. Add fields to the table

**Goal:** give the `Product` table some fields (columns).
**Before you start:** you finished [tutorial 2](02-create-a-table.md).
**Time:** ~4 minutes.

A field is an **attribute**. Each one has a type that decides what it stores
and how it shows up in forms. We'll add two: a product name and a quantity.

## Add the name field

1. Open **Tools → Database Schema** and click `Product`.
2. Under **Attributes**, click **New attribute**.
3. Fill in:
   - **Label** — `Name`
   - **Machine name** — fills in as `name`
   - **Type** — choose **TEXT - Text**
4. Check **Required field** — a product must have a name.
5. Click **Create attribute** (at the bottom).

You return to the `Product` page and `name` now appears under **Attributes**.

## Add the quantity field

1. Click **New attribute** again.
2. Fill in:
   - **Label** — `Quantity`
   - **Machine name** — `quantity`
   - **Type** — choose **INT - Integer number**
3. Leave it optional this time. Click **Create attribute**.

## Check it worked

The `Product` page lists both attributes — `name` (TEXT, required) and
`quantity` (INT).

## The five types

When you pick a **Type**, these are your choices:

| Type | Stores | Example |
|------|--------|---------|
| TEXT | text | a name, a description |
| INT | whole numbers | a quantity, a count |
| REAL | decimal numbers | a price, a weight |
| BOOL | yes / no | in stock? |
| DATETIME | a date and time | received on |

The type is fixed once the attribute exists — pick it deliberately.

## Next

→ [4. Create a form for the table](04-create-a-form.md)
