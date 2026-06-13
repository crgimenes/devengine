# 5. Use the form

**Goal:** create, list and edit records through the form you built.
**Before you start:** you finished [tutorial 4](04-create-a-form.md).
**Time:** ~3 minutes.

So far you worked in **Tools** — the builder. Now you'll use the app the way an
end user does: through the *runtime* screen at `/form/products`.

## Add a product

1. Go to `/form/products` (or click **Try it** on the form's edit page).
2. Type a **Name**, e.g. `Wireless mouse`, and a **Quantity**, e.g. `40`.
3. Click **Create record**.

You're redirected to the record in edit mode and a green
*"Record created successfully"* banner appears.

## See the list

Go to `/form/products/list`. Your product shows up in a table with **Name** and
**Quantity** columns — the columns are exactly the fields you put on the form.

Add one or two more from the **+ New** button so the list isn't lonely.

## Edit a product

1. On the listing, click **Open** on a row.
2. Change the **Quantity** and click **Save changes**.
3. Back on the listing, the new value is there.

## Try the required rule

Open `/form/products`, leave **Name** empty, and click **Create record**. The
form comes back with the value you typed preserved and the Name field flagged —
the *Required* setting from tutorial 2 doing its job. Fill the name and it
saves.

## Two URLs to remember

- `/form/products` — the form to **create** a new record.
- `/form/products/list` — the **listing** of existing records (with search).

## Next

→ [6. Create a menu](06-create-a-menu.md) — so users don't have to type URLs.
