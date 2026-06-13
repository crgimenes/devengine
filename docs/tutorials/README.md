# devengine tutorials

Short, hands-on lessons that build on each other. Each one is a single small
task you can finish in a few minutes. Do them in order — every tutorial picks
up the app you built in the previous one.

We build one running example throughout: a small **inventory** — products,
suppliers, and the screens to manage them.

## Before you begin

- **Go 1.26+** installed, and the `devengine` and `filo` repositories checked
  out on disk. [Tutorial 1](01-create-an-application.md) builds your app against
  them and takes you from nothing to a running app with a sysop login — start
  there.
- These tutorials use the **English** interface. To match the labels exactly,
  set your profile language to English (tutorial 1 shows where). Everything
  still works in any language; only the button names differ.

From tutorial 2 on, everything happens under **Tools** (the sidebar on the left
after you sign in). Those screens build and maintain the app; your end users
live in the *runtime* screens (`/form/...`) you'll wire up along the way.

## The path

**Start here**

1. [Create a new application](01-create-an-application.md) — your own app on devengine

**Build your first feature**

2. [Create your first table](02-create-a-table.md)
3. [Add fields to the table](03-add-fields.md)
4. [Create a form for the table](04-create-a-form.md)
5. [Use the form](05-use-the-form.md) — create, list and edit records
6. [Create a menu](06-create-a-menu.md) — give users a way in

**Make it real** (coming next)

7. Required fields and default values
8. A list of options (the select field)
9. Validate what users type
10. A calculated field
11. Relate two tables (reference)
12. A sub-form (related items)
13. A search form
14. An action button
15. Group fields (cards and tabs)
16. Expose the form as a REST API
17. Extras — language, files, backups
