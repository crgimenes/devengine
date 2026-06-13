# 4. Create a form for the table

**Goal:** build a screen where someone can enter and edit a product.
**Before you start:** you finished [tutorial 3](03-add-fields.md).
**Time:** ~4 minutes.

A table holds data; a **form** is how people put data in and read it back. One
table can have several forms (a full editor, a quick-add, a search). We'll
build one form over `Product`.

## Create the form

1. Open **Tools → Forms** and click **New form** (top right).
2. Fill in:
   - **Label** — `Products`
   - **Machine name** — fills in as `products` (this becomes the URL,
     `/form/products`)
   - **EAV table** — choose `Product`. This is what binds the form to your
     table so it actually saves records.
3. Click **Create form**.

You land on the form's edit page. It has no fields yet.

## Add the fields to the form

Scroll to **Form elements**. It says *"No elements added yet."* Use the
**Add element** row at the bottom:

1. Under **EAV field**, choose `name`. The Label and Name fill in for you.
2. Click the **＋** (add) button.
3. Do the same for `quantity`: choose it under **EAV field**, click **＋**.

`name` and `quantity` now appear in the **Form elements** list.

## Check it worked

At the top of the form page, click **Try it** (the green button with a play
icon). A new tab opens at `/form/products` showing your form: a **Name** box
(marked required) and a **Quantity** box. Don't fill it in yet — that's the
next tutorial.

## Why two steps (table, then form)?

The table decides what can be stored; the form decides what this particular
screen shows and in what order. Keeping them separate is what lets you point
several different screens — a full editor, a search, a public sign-up — at the
same data.

## Next

→ [5. Use the form](05-use-the-form.md)
