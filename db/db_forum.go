package db

import (
	"github.com/crgimenes/devengine/log"
)

// GetForumByExternalID retrieves a forum by its external ID.
func (s *SQLite) GetForumByExternalID(externalID string) (*Forum, error) {
	const sqlSelect = `SELECT
        f.id,                       -- 1
        f.external_id,              -- 2
        f.tenant_id,                -- 3
        f.workspace_id,             -- 4
        f.owner_user_id,            -- 5
        COALESCE(u.username, ''),   -- 6
        COALESCE(f.title, ''),      -- 7
        COALESCE(f.description, ''),-- 8
        COALESCE(f.image_url, ''),  -- 9
        f.created_at,               -- 10
        f.updated_at                -- 11
    FROM forum f
    LEFT JOIN users u ON f.owner_user_id = u.id
    WHERE f.external_id = ?  -- 1
    LIMIT 1`

	var f Forum

	err := s.QueryRow(
		sqlSelect,
		externalID, // 1
	).Scan(
		&f.ID,            // 1
		&f.ExternalID,    // 2
		&f.TenantID,      // 3
		&f.WorkspaceID,   // 4
		&f.OwnerUserID,   // 5
		&f.OwnerUserName, // 6
		&f.Title,         // 7
		&f.Description,   // 8
		&f.ImageURL,      // 9
		&f.CreatedAt,     // 10
		&f.UpdatedAt,     // 11
	)
	if err != nil {
		log.Printf("DEBUG: GetForumByExternalID query error for external_id=%s: %v", externalID, err)
		return nil, err
	}

	log.Printf("DEBUG: GetForumByExternalID found forum - id=%d, external_id=%s, title=%s", f.ID, f.ExternalID, f.Title)
	return &f, nil
}

// CreateThread creates a new thread in a forum.
func (s *SQLite) CreateThread(
	forumID int64,
	ownerUserID int64,
	title string,
	externalID string,
	imageURL string,
) (*Thread, error) {
	const sqlInsert = `INSERT INTO forum_threads (
        external_id,       -- 1
        forum_id,          -- 2
        owner_user_id,     -- 3
        title,             -- 4
        image_url,         -- 5
        created_at,
        updated_at
    ) VALUES (
        ?,                 -- 1
        ?,                 -- 2
        ?,                 -- 3
        ?,                 -- 4
        ?,                 -- 5
        CURRENT_TIMESTAMP, -- created_at
        CURRENT_TIMESTAMP  -- updated_at
    )
    RETURNING
        id,                        -- 1
        external_id,               -- 2
        forum_id,                  -- 3
        owner_user_id,             -- 4
        COALESCE(title, ''),       -- 5
        COALESCE(image_url, ''),   -- 6
        created_at,                -- 7
        updated_at` // 8

	var t Thread

	err := s.QueryRowRW(
		sqlInsert,
		externalID,  // 1
		forumID,     // 2
		ownerUserID, // 3
		title,       // 4
		imageURL,    // 5
	).Scan(
		&t.ID,          // 1
		&t.ExternalID,  // 2
		&t.ForumID,     // 3
		&t.OwnerUserID, // 4
		&t.Title,       // 5
		&t.ImageURL,    // 6
		&t.CreatedAt,   // 7
		&t.UpdatedAt,   // 8
	)
	if err != nil {
		log.Printf("ERROR: CreateThread INSERT failed for forumID=%d, ownerID=%d: %v", forumID, ownerUserID, err)
		return nil, err
	}

	log.Printf("DEBUG: CreateThread INSERT succeeded, got thread id=%d", t.ID)

	// Fetch the username from the users table
	const sqlSelectUsername = `SELECT COALESCE(username, '') FROM users WHERE id = ? LIMIT 1`
	err = s.QueryRow(sqlSelectUsername, ownerUserID).Scan(&t.OwnerUserName)
	if err != nil {
		// If we can't fetch the username, just leave it empty (user might not exist yet)
		log.Printf("DEBUG: CreateThread username lookup failed for ownerID=%d: %v (will use empty string)", ownerUserID, err)
		t.OwnerUserName = ""
	}

	log.Printf("DEBUG: CreateThread complete - id=%d, external_id=%s, forum_id=%d", t.ID, t.ExternalID, t.ForumID)
	return &t, nil
}

// CreatePost creates a new post in a thread.
func (s *SQLite) CreatePost(
	threadID int64,
	ownerUserID int64,
	content string,
	contentHTML string,
	externalID string,
	parentPostID *int64,
) (*Post, error) {
	const sqlInsert = `INSERT INTO forum_posts (
        external_id,       -- 1
        thread_id,         -- 2
        owner_user_id,     -- 3
        content,           -- 4
        content_html,      -- 5
        parent_post_id,    -- 6
        created_at,
        updated_at
    ) VALUES (
        ?,                 -- 1
        ?,                 -- 2
        ?,                 -- 3
        ?,                 -- 4
        ?,                 -- 5
        ?,                 -- 6
        CURRENT_TIMESTAMP, -- created_at
        CURRENT_TIMESTAMP  -- updated_at
    )
    RETURNING
        id,                         -- 1
        external_id,                -- 2
        thread_id,                  -- 3
        owner_user_id,              -- 4
        COALESCE(content, ''),      -- 5
        COALESCE(content_html, ''), -- 6
        parent_post_id,             -- 7
        created_at,                 -- 8
        updated_at` // 9

	var p Post

	err := s.QueryRowRW(
		sqlInsert,
		externalID,   // 1
		threadID,     // 2
		ownerUserID,  // 3
		content,      // 4
		contentHTML,  // 5
		parentPostID, // 6
	).Scan(
		&p.ID,           // 1
		&p.ExternalID,   // 2
		&p.ThreadID,     // 3
		&p.OwnerUserID,  // 4
		&p.Content,      // 5
		&p.ContentHTML,  // 6
		&p.ParentPostID, // 7
		&p.CreatedAt,    // 8
		&p.UpdatedAt,    // 9
	)
	if err != nil {
		log.Printf("ERROR: CreatePost INSERT failed for threadID=%d, ownerID=%d: %v", threadID, ownerUserID, err)
		return nil, err
	}

	log.Printf("DEBUG: CreatePost INSERT succeeded - id=%d, external_id=%s, thread_id=%d", p.ID, p.ExternalID, p.ThreadID)
	return &p, nil
}

// UpdatePost updates an existing post's content.
func (s *SQLite) UpdatePost(
	postID int64,
	content string,
	contentHTML string,
) (*Post, error) {
	const sqlUpdate = `UPDATE forum_posts
    SET
        content = ?,               -- 1
        content_html = ?,          -- 2
        updated_at = CURRENT_TIMESTAMP
    WHERE id = ?                   -- 3
    RETURNING
        id,                         -- 1
        external_id,                -- 2
        thread_id,                  -- 3
        owner_user_id,              -- 4
        COALESCE(content, ''),      -- 5
        COALESCE(content_html, ''), -- 6
        parent_post_id,             -- 7
        created_at,                 -- 8
        updated_at` // 9

	var p Post

	err := s.QueryRowRW(
		sqlUpdate,
		content,     // 1
		contentHTML, // 2
		postID,      // 3
	).Scan(
		&p.ID,           // 1
		&p.ExternalID,   // 2
		&p.ThreadID,     // 3
		&p.OwnerUserID,  // 4
		&p.Content,      // 5
		&p.ContentHTML,  // 6
		&p.ParentPostID, // 7
		&p.CreatedAt,    // 8
		&p.UpdatedAt,    // 9
	)
	if err != nil {
		return nil, err
	}

	return &p, nil
}

// DeletePost deletes a post by ID.
func (s *SQLite) DeletePost(postID int64) error {
	const sqlDelete = `DELETE FROM forum_posts
        WHERE id = ?` // 1

	err := s.Exec(
		sqlDelete,
		postID, // 1
	)
	return err
}

// GetThreadByExternalID retrieves a thread by its external ID.
func (s *SQLite) GetThreadByExternalID(externalID string) (*Thread, error) {
	const sqlSelect = `SELECT
        t.id,                      -- 1
        t.external_id,             -- 2
        t.forum_id,                -- 3
        t.owner_user_id,           -- 4
        COALESCE(t.title, ''),     -- 5
        COALESCE(t.image_url, ''), -- 6
        COALESCE(u.username, ''),  -- 7
        t.created_at,              -- 8
        t.updated_at               -- 9
    FROM forum_threads t
    LEFT JOIN users u ON t.owner_user_id = u.id
    WHERE t.external_id = ?  -- 1
    LIMIT 1`

	var t Thread

	err := s.QueryRow(
		sqlSelect,
		externalID, // 1
	).Scan(
		&t.ID,            // 1
		&t.ExternalID,    // 2
		&t.ForumID,       // 3
		&t.OwnerUserID,   // 4
		&t.Title,         // 5
		&t.ImageURL,      // 6
		&t.OwnerUserName, // 7
		&t.CreatedAt,     // 8
		&t.UpdatedAt,     // 9
	)
	if err != nil {
		return nil, err
	}

	return &t, nil
}

// ListThreadsByForumID retrieves threads in a forum with pagination.
func (s *SQLite) ListThreadsByForumID(
	forumID int64,
	offset int,
	limit int,
) ([]*Thread, error) {
	const sqlSelect = `SELECT
        t.id,                      -- 1
        t.external_id,             -- 2
        t.forum_id,                -- 3
        t.owner_user_id,           -- 4
        COALESCE(t.title, ''),     -- 5
        COALESCE(t.image_url, ''), -- 6
        COALESCE(u.username, ''),  -- 7
        t.created_at,              -- 8
        t.updated_at               -- 9
    FROM forum_threads t
    LEFT JOIN users u ON t.owner_user_id = u.id
    WHERE t.forum_id = ?       -- 1
    ORDER BY t.created_at DESC
    LIMIT ?                    -- 2
    OFFSET ?` // 3

	rows, err := s.Query(
		sqlSelect,
		forumID, // 1
		limit,   // 2
		offset,  // 3
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var threads []*Thread
	for rows.Next() {
		var t Thread
		if err := rows.Scan(
			&t.ID,            // 1
			&t.ExternalID,    // 2
			&t.ForumID,       // 3
			&t.OwnerUserID,   // 4
			&t.Title,         // 5
			&t.ImageURL,      // 6
			&t.OwnerUserName, // 7
			&t.CreatedAt,     // 8
			&t.UpdatedAt,     // 9
		); err != nil {
			return nil, err
		}
		threads = append(threads, &t)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return threads, nil
}

// UpdateThread updates an existing thread's title, image_url, and updated_at timestamp.
func (s *SQLite) UpdateThread(
	externalID string,
	title string,
	imageURL string,
) (*Thread, error) {
	const sqlUpdate = `UPDATE forum_threads
    SET
        title = ?,                 -- 1
        image_url = ?,             -- 2
        updated_at = CURRENT_TIMESTAMP
    WHERE external_id = ?          -- 3
    RETURNING
        id,                        -- 1
        external_id,               -- 2
        forum_id,                  -- 3
        owner_user_id,             -- 4
        COALESCE(title, ''),       -- 5
        COALESCE(image_url, ''),   -- 6
        created_at,                -- 7
        updated_at` // 8

	var t Thread

	err := s.QueryRowRW(
		sqlUpdate,
		title,      // 1
		imageURL,   // 2
		externalID, // 3
	).Scan(
		&t.ID,          // 1
		&t.ExternalID,  // 2
		&t.ForumID,     // 3
		&t.OwnerUserID, // 4
		&t.Title,       // 5
		&t.ImageURL,    // 6
		&t.CreatedAt,   // 7
		&t.UpdatedAt,   // 8
	)
	if err != nil {
		return nil, err
	}

	return &t, nil
}

// GetPostsByThreadID retrieves posts in a thread.
func (s *SQLite) GetPostsByThreadID(threadID int64) ([]*Post, error) {
	const sqlSelect = `SELECT
        id,                         -- 1
        external_id,                -- 2
        thread_id,                  -- 3
        owner_user_id,              -- 4
        COALESCE(content, ''),      -- 5
        COALESCE(content_html, ''), -- 6
        parent_post_id,             -- 7
        created_at,                 -- 8
        updated_at                  -- 9
    FROM forum_posts
    WHERE thread_id = ?             -- 1
    AND parent_post_id IS NULL
    ORDER BY created_at ASC`

	rows, err := s.Query(
		sqlSelect,
		threadID, // 1
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var posts []*Post
	for rows.Next() {
		var p Post
		if err := rows.Scan(
			&p.ID,           // 1
			&p.ExternalID,   // 2
			&p.ThreadID,     // 3
			&p.OwnerUserID,  // 4
			&p.Content,      // 5
			&p.ContentHTML,  // 6
			&p.ParentPostID, // 7
			&p.CreatedAt,    // 8
			&p.UpdatedAt,    // 9
		); err != nil {
			return nil, err
		}
		posts = append(posts, &p)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return posts, nil
}

// ListForums retrieves all forums with pagination.
func (s *SQLite) ListForums(offset int, limit int) ([]*Forum, error) {
	const sqlSelect = `SELECT
        f.id,                        -- 1
        f.external_id,               -- 2
        f.tenant_id,                 -- 3
        f.workspace_id,              -- 4
        f.owner_user_id,             -- 5
        COALESCE(u.username, ''),    -- 6
        COALESCE(f.title, ''),       -- 7
        COALESCE(f.description, ''), -- 8
        COALESCE(f.image_url, ''),   -- 9
        f.created_at,                -- 10
        f.updated_at                 -- 11
    FROM forum f
    LEFT JOIN users u ON f.owner_user_id = u.id
    ORDER BY f.created_at DESC
    LIMIT ?                      -- 1
    OFFSET ?` // 2

	rows, err := s.Query(
		sqlSelect,
		limit,  // 1
		offset, // 2
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var forums []*Forum
	for rows.Next() {
		var f Forum
		if err := rows.Scan(
			&f.ID,            // 1
			&f.ExternalID,    // 2
			&f.TenantID,      // 3
			&f.WorkspaceID,   // 4
			&f.OwnerUserID,   // 5
			&f.OwnerUserName, // 6
			&f.Title,         // 7
			&f.Description,   // 8
			&f.ImageURL,      // 9
			&f.CreatedAt,     // 10
			&f.UpdatedAt,     // 11
		); err != nil {
			return nil, err
		}
		forums = append(forums, &f)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return forums, nil
}

// UpdateForum updates an existing forum.
func (s *SQLite) UpdateForum(
	externalID string,
	title string,
	description string,
	imageURL string,
) (*Forum, error) {
	const sqlUpdate = `UPDATE forum
    SET
        title = ?,                 -- 1
        description = ?,           -- 2
        image_url = ?,             -- 3
        updated_at = CURRENT_TIMESTAMP
    WHERE external_id = ?          -- 4
    RETURNING
        id,                         -- 1
        external_id,                -- 2
        tenant_id,                  -- 3
        workspace_id,               -- 4
        owner_user_id,              -- 5
        COALESCE(title, ''),        -- 6
        COALESCE(description, ''),  -- 7
        COALESCE(image_url, ''),    -- 8
        created_at,                 -- 9
        updated_at` // 10

	var f Forum

	err := s.QueryRowRW(
		sqlUpdate,
		title,       // 1
		description, // 2
		imageURL,    // 3
		externalID,  // 4
	).Scan(
		&f.ID,          // 1
		&f.ExternalID,  // 2
		&f.TenantID,    // 3
		&f.WorkspaceID, // 4
		&f.OwnerUserID, // 5
		&f.Title,       // 6
		&f.Description, // 7
		&f.ImageURL,    // 8
		&f.CreatedAt,   // 9
		&f.UpdatedAt,   // 10
	)
	if err != nil {
		return nil, err
	}

	return &f, nil
}

// CreateForum creates a new forum.
func (s *SQLite) CreateForum(
	externalID string,
	tenantID int64,
	workspaceID int64,
	ownerUserID int64,
	title string,
	description string,
	imageURL string,
) (*Forum, error) {
	const sqlInsert = `INSERT INTO forum (
        external_id,       -- 1
        tenant_id,         -- 2
        workspace_id,      -- 3
        owner_user_id,     -- 4
        title,             -- 5
        description,       -- 6
        image_url,         -- 7
        created_at,
        updated_at
    ) VALUES (
        ?,                 -- 1
        ?,                 -- 2
        ?,                 -- 3
        ?,                 -- 4
        ?,                 -- 5
        ?,                 -- 6
        ?,                 -- 7
        CURRENT_TIMESTAMP, -- created_at
        CURRENT_TIMESTAMP  -- updated_at
    )
    RETURNING
        id,                         -- 1
        external_id,                -- 2
        tenant_id,                  -- 3
        workspace_id,               -- 4
        owner_user_id,              -- 5
        COALESCE(title, ''),        -- 6
        COALESCE(description, ''),  -- 7
        COALESCE(image_url, ''),    -- 8
        created_at,                 -- 9
        updated_at` // 10

	var f Forum

	err := s.QueryRowRW(
		sqlInsert,
		externalID,  // 1
		tenantID,    // 2
		workspaceID, // 3
		ownerUserID, // 4
		title,       // 5
		description, // 6
		imageURL,    // 7
	).Scan(
		&f.ID,          // 1
		&f.ExternalID,  // 2
		&f.TenantID,    // 3
		&f.WorkspaceID, // 4
		&f.OwnerUserID, // 5
		&f.Title,       // 6
		&f.Description, // 7
		&f.ImageURL,    // 8
		&f.CreatedAt,   // 9
		&f.UpdatedAt,   // 10
	)
	if err != nil {
		return nil, err
	}

	// Fetch the username from the users table
	const sqlSelectUsername = `SELECT COALESCE(username, '') FROM users WHERE id = ? LIMIT 1`
	err = s.QueryRow(sqlSelectUsername, ownerUserID).Scan(&f.OwnerUserName)
	if err != nil {
		// If we can't fetch the username, just leave it empty (user might not exist yet)
		f.OwnerUserName = ""
	}

	return &f, nil
}
