# 6. Create a menu

**Goal:** add a navigation menu so users reach the Products screen by clicking,
not by typing a URL.
**Before you start:** you finished [tutorial 5](05-use-the-form.md).
**Time:** ~4 minutes.

A **menu** is the bar of links across the top of the app. You build the menu,
add items that point at your screens, then attach the menu to a form so it
shows up while that form is in use.

## Create the menu

1. Open **Tools → Menu Editor** and click **New menu**.
2. Fill in:
   - **Name** — `Main menu`
   - **Machine name** — fills in as `main_menu`
3. Click **Create menu**.

## Add an item

You land on the menu's edit page. Under **Add a new item**:

1. **Label** — `Products`
2. **Machine name** — fills in as `products`
3. Leave **Icon** and **Parent item** (it defaults to **Root**, a top-level
   item).
4. Click **Add**.

The item opens for editing. Set where it points:

1. **Type** — `Link`.
2. **URL for direct navigation** — `/form/products/list`.
3. Click **Save** (or **Save changes**).

## Attach the menu to the form

A menu only appears while a form that uses it is active. Wire them together:

1. Open **Tools → Forms** and click `Products`.
2. Find the **Menu** dropdown (*"Menu shown on the navbar while this form is
   active."*) and choose `Main menu`.
3. Click **Save changes**.

## Check it worked

Go to `/form/products/list`. The top bar now shows a **Products** link. Click
around — it stays put as you create and edit records. Your users never have to
know a URL.

## What you've built

From an empty app, five short steps gave you a working feature: a table, its
fields, a form over it, real records, and a menu to reach them — no SQL, no
deploy. The next tutorials add the pieces that make it feel finished: defaults,
option lists, validation, calculated fields, relationships, and more.

## Next

→ Back to the [tutorial index](README.md) for what's coming.
