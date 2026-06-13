# 2. Create your first table

**Goal:** create a table to hold your data.
**Before you start:** you finished [tutorial 1](01-create-an-application.md).
**Time:** ~2 minutes.

A *table* in devengine is an **entity type** — a named place to keep records of
the same kind. You define the table once, add fields to it (next tutorial),
then build a form over it. We'll make a `Product` table for our inventory.

## Steps

1. In the left sidebar, open **Tools → Database Schema**.
2. Click **New EAV Table** (top right).
3. Fill in:
   - **Name** — `Product`
   - **Machine name** — leave it. It fills in as `product` while you type.
     This is the table's identifier in URLs and scripts; lowercase, no spaces.
4. Leave **Description** and the two Filo script boxes empty for now.
5. Click **Create table**.

## Check it worked

You land on the new table's page. Under **Attributes** you'll see
*"No attributes yet. Click 'New attribute' to add fields to this table."* —
that's expected; the table has no fields yet.

Open **Tools → Database Schema** again: `Product` now shows up in the list.

## What just happened

You created an empty table. devengine stores its records in a flexible
entity-attribute-value layout, so you never write SQL or run a migration to
add a table — you just define it here and start adding fields.

## Next

→ [3. Add fields to the table](03-add-fields.md)
