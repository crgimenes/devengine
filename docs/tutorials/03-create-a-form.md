# 3. Create a form and use it

**Goal:** build a screen over the `Product` table, then create, list and edit
records through it.
**Before you start:** you finished [tutorial 2](02-create-a-table.md).
**Time:** ~7 minutes.

A table holds data; a **form** is how people put data in and read it back. One
table can have several forms (a full editor, a quick-add, a search). We'll build
one form over `Product`, then use it the way an end user does.

## Create the form

1. Open **Tools → Forms** and click **New form** (top right).
2. Fill in:
   - **Label** — `Products`
   - **Machine name** — fills in as `products` (this becomes the URL,
     `/form/products`)
   - **EAV table** — choose `Product`. This binds the form to your table so it
     actually saves records.
3. Click **Create form**.

You land on the form's edit page. It has no fields yet.

## Add the fields to the form

Scroll to **Form elements**. It says *"No elements added yet."* Use the
**Add element** row at the bottom. For each field:

1. Under **EAV field**, choose the field — the Label and Name fill in for you.
2. Click the **＋** (add) button.

Add four of them, in order: `name`, `quantity`, `price`, `in_stock`.

Leave `received_on` off **on purpose**. The table has five fields, but this
screen only needs four — a form decides what *this* screen shows, independent of
what the table can store. That's what lets you point a full editor, a search,
and a public sign-up at the same data.

The four fields now appear in the **Form elements** list.

## Add a product

At the top of the form page, click **Try it** (the green button with a play
icon), or just go to `/form/products`. You're now using the *runtime* — the app
as an end user sees it, not the builder.

1. Fill in **Name** `Wireless mouse`, **Quantity** `40`, **Price** `29.90`, and
   tick **In stock**.
2. Click **Create record**.

You're redirected to the record in edit mode and a green
*"Record created successfully"* banner appears.

## See the list

Go to `/form/products/list`. Your product shows up in a table whose columns are
exactly the fields you put on the form. Add one or two more from the **+ New**
button so the list isn't lonely.

## Edit a product

1. On the listing, click **Open** on a row.
2. Change the **Quantity** and click **Save changes**.
3. Back on the listing, the new value is there.

## Try the required rule

Open `/form/products`, leave **Name** empty, and click **Create record**. The
form comes back with the values you typed preserved and the Name field flagged —
the *Required* setting from tutorial 2 doing its job. Fill the name and it saves.

## Two URLs to remember

- `/form/products` — the form to **create** a new record.
- `/form/products/list` — the **listing** of existing records (with search).

## Next

→ [4. Create a menu](04-create-a-menu.md) — so users don't have to type URLs.
