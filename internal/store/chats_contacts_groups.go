package store

import (
	"fmt"
	"strings"
	"time"
)

type ChatListFilter struct {
	Query    string
	Limit    int
	Archived *bool
	Pinned   *bool
	Muted    *bool
	Unread   *bool
}

func (d *DB) UpsertChat(jid, kind, name string, lastTS time.Time) error {
	if strings.TrimSpace(kind) == "" {
		kind = "unknown"
	}
	_, err := d.sql.Exec(`
		INSERT INTO chats(jid, kind, name, last_message_ts)
		VALUES(?, ?, ?, ?)
		ON CONFLICT(jid) DO UPDATE SET
			kind=excluded.kind,
			name=CASE WHEN excluded.name IS NOT NULL AND excluded.name != '' THEN excluded.name ELSE chats.name END,
			last_message_ts=CASE WHEN excluded.last_message_ts > COALESCE(chats.last_message_ts, 0) THEN excluded.last_message_ts ELSE chats.last_message_ts END
	`, jid, kind, name, unix(lastTS))
	return err
}

func (d *DB) ListChats(f ChatListFilter) ([]Chat, error) {
	if f.Limit <= 0 {
		f.Limit = 50
	}
	q := `SELECT jid, kind, COALESCE(name,''), COALESCE(last_message_ts,0), COALESCE(archived,0), COALESCE(pinned,0), COALESCE(muted_until,0), COALESCE(unread,0) FROM chats WHERE 1=1`
	var args []interface{}
	if strings.TrimSpace(f.Query) != "" {
		q += ` AND (LOWER(name) LIKE LOWER(?) OR LOWER(jid) LIKE LOWER(?))`
		needle := "%" + f.Query + "%"
		args = append(args, needle, needle)
	}
	if f.Archived != nil {
		q += ` AND archived = ?`
		args = append(args, boolToInt(*f.Archived))
	}
	if f.Pinned != nil {
		q += ` AND pinned = ?`
		args = append(args, boolToInt(*f.Pinned))
	}
	if f.Muted != nil {
		if *f.Muted {
			q += ` AND muted_until != 0 AND (muted_until = -1 OR muted_until > ?)`
			args = append(args, time.Now().Unix())
		} else {
			q += ` AND (muted_until = 0 OR (muted_until > 0 AND muted_until <= ?))`
			args = append(args, time.Now().Unix())
		}
	}
	if f.Unread != nil {
		q += ` AND unread = ?`
		args = append(args, boolToInt(*f.Unread))
	}
	q += ` ORDER BY pinned DESC, last_message_ts DESC LIMIT ?`
	args = append(args, f.Limit)

	rows, err := d.sql.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Chat
	for rows.Next() {
		var c Chat
		var ts int64
		var archived, pinned, unread int
		if err := rows.Scan(&c.JID, &c.Kind, &c.Name, &ts, &archived, &pinned, &c.MutedUntil, &unread); err != nil {
			return nil, err
		}
		c.LastMessageTS = fromUnix(ts)
		c.Archived = archived != 0
		c.Pinned = pinned != 0
		c.Unread = unread != 0
		out = append(out, c)
	}
	return out, rows.Err()
}

func (d *DB) GetChat(jid string) (Chat, error) {
	row := d.sql.QueryRow(`SELECT jid, kind, COALESCE(name,''), COALESCE(last_message_ts,0), COALESCE(archived,0), COALESCE(pinned,0), COALESCE(muted_until,0), COALESCE(unread,0) FROM chats WHERE jid = ?`, jid)
	var c Chat
	var ts int64
	var archived, pinned, unread int
	if err := row.Scan(&c.JID, &c.Kind, &c.Name, &ts, &archived, &pinned, &c.MutedUntil, &unread); err != nil {
		return Chat{}, err
	}
	c.LastMessageTS = fromUnix(ts)
	c.Archived = archived != 0
	c.Pinned = pinned != 0
	c.Unread = unread != 0
	return c, nil
}

func (d *DB) SetChatArchived(jid string, v bool) error {
	_, err := d.sql.Exec(`UPDATE chats SET archived = ? WHERE jid = ?`, boolToInt(v), jid)
	return err
}

func (d *DB) SetChatPinned(jid string, v bool) error {
	_, err := d.sql.Exec(`UPDATE chats SET pinned = ? WHERE jid = ?`, boolToInt(v), jid)
	return err
}

func (d *DB) SetChatMutedUntil(jid string, v int64) error {
	_, err := d.sql.Exec(`UPDATE chats SET muted_until = ? WHERE jid = ?`, v, jid)
	return err
}

func (d *DB) SetChatUnread(jid string, v bool) error {
	_, err := d.sql.Exec(`UPDATE chats SET unread = ? WHERE jid = ?`, boolToInt(v), jid)
	return err
}

func (d *DB) SearchContacts(query string, limit int) ([]Contact, error) {
	if strings.TrimSpace(query) == "" {
		return nil, fmt.Errorf("query is required")
	}
	if limit <= 0 {
		limit = 50
	}
	q := `
		SELECT c.jid,
		       COALESCE(c.phone,''),
		       COALESCE(NULLIF(a.alias,''), ''),
		       COALESCE(NULLIF(c.full_name,''), NULLIF(c.push_name,''), NULLIF(c.business_name,''), NULLIF(c.first_name,''), ''),
		       c.updated_at
		FROM contacts c
		LEFT JOIN contact_aliases a ON a.jid = c.jid
		WHERE LOWER(COALESCE(a.alias,'')) LIKE LOWER(?) OR LOWER(COALESCE(c.full_name,'')) LIKE LOWER(?) OR LOWER(COALESCE(c.push_name,'')) LIKE LOWER(?) OR LOWER(COALESCE(c.phone,'')) LIKE LOWER(?) OR LOWER(c.jid) LIKE LOWER(?)
		ORDER BY COALESCE(NULLIF(a.alias,''), NULLIF(c.full_name,''), NULLIF(c.push_name,''), c.jid)
		LIMIT ?`
	needle := "%" + query + "%"
	rows, err := d.sql.Query(q, needle, needle, needle, needle, needle, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Contact
	for rows.Next() {
		var c Contact
		var updated int64
		if err := rows.Scan(&c.JID, &c.Phone, &c.Alias, &c.Name, &updated); err != nil {
			return nil, err
		}
		c.UpdatedAt = fromUnix(updated)
		out = append(out, c)
	}
	return out, rows.Err()
}

func (d *DB) GetContact(jid string) (Contact, error) {
	row := d.sql.QueryRow(`
		SELECT c.jid,
		       COALESCE(c.phone,''),
		       COALESCE(NULLIF(a.alias,''), ''),
		       COALESCE(NULLIF(c.full_name,''), NULLIF(c.push_name,''), NULLIF(c.business_name,''), NULLIF(c.first_name,''), ''),
		       c.updated_at
		FROM contacts c
		LEFT JOIN contact_aliases a ON a.jid = c.jid
		WHERE c.jid = ?
	`, jid)
	var c Contact
	var updated int64
	if err := row.Scan(&c.JID, &c.Phone, &c.Alias, &c.Name, &updated); err != nil {
		return Contact{}, err
	}
	c.UpdatedAt = fromUnix(updated)
	tags, _ := d.ListTags(jid)
	c.Tags = tags
	return c, nil
}

func (d *DB) ListTags(jid string) ([]string, error) {
	rows, err := d.sql.Query(`SELECT tag FROM contact_tags WHERE jid = ? ORDER BY tag`, jid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []string
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}
	return tags, rows.Err()
}

func (d *DB) UpsertContact(jid, phone, pushName, fullName, firstName, businessName string) error {
	now := time.Now().UTC().Unix()
	_, err := d.sql.Exec(`
		INSERT INTO contacts(jid, phone, push_name, full_name, first_name, business_name, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(jid) DO UPDATE SET
			phone=COALESCE(NULLIF(excluded.phone,''), contacts.phone),
			push_name=COALESCE(NULLIF(excluded.push_name,''), contacts.push_name),
			full_name=COALESCE(NULLIF(excluded.full_name,''), contacts.full_name),
			first_name=COALESCE(NULLIF(excluded.first_name,''), contacts.first_name),
			business_name=COALESCE(NULLIF(excluded.business_name,''), contacts.business_name),
			updated_at=excluded.updated_at
	`, jid, phone, pushName, fullName, firstName, businessName, now)
	return err
}

func (d *DB) UpsertGroup(jid, name, ownerJID string, created time.Time, isParent bool, linkedParentJID string) error {
	now := time.Now().UTC().Unix()
	_, err := d.sql.Exec(`
		INSERT INTO groups(jid, name, owner_jid, created_ts, is_parent, linked_parent_jid, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(jid) DO UPDATE SET
			name=COALESCE(NULLIF(excluded.name,''), groups.name),
			owner_jid=COALESCE(NULLIF(excluded.owner_jid,''), groups.owner_jid),
			created_ts=COALESCE(NULLIF(excluded.created_ts,0), groups.created_ts),
			is_parent=excluded.is_parent,
			linked_parent_jid=COALESCE(NULLIF(excluded.linked_parent_jid,''), groups.linked_parent_jid),
			updated_at=excluded.updated_at
	`, jid, name, ownerJID, unix(created), isParent, linkedParentJID, now)
	return err
}

func (d *DB) ReplaceGroupParticipants(groupJID string, participants []GroupParticipant) (err error) {
	tx, err := d.sql.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if _, err = tx.Exec(`DELETE FROM group_participants WHERE group_jid = ?`, groupJID); err != nil {
		return err
	}
	stmt, err := tx.Prepare(`INSERT INTO group_participants(group_jid, user_jid, role, updated_at) VALUES(?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	now := time.Now().UTC()
	for _, participant := range participants {
		role := strings.TrimSpace(participant.Role)
		if role == "" {
			role = "member"
		}
		if _, err = stmt.Exec(groupJID, participant.UserJID, role, unix(now)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (d *DB) ListGroups(query string, limit int) ([]Group, error) {
	if limit <= 0 {
		limit = 50
	}
	q := `SELECT jid, COALESCE(name,''), COALESCE(owner_jid,''), COALESCE(created_ts,0), updated_at, COALESCE(is_parent,0), COALESCE(linked_parent_jid,'') FROM groups WHERE 1=1`
	var args []interface{}
	if strings.TrimSpace(query) != "" {
		needle := "%" + query + "%"
		q += ` AND (LOWER(name) LIKE LOWER(?) OR LOWER(jid) LIKE LOWER(?))`
		args = append(args, needle, needle)
	}
	q += ` ORDER BY COALESCE(created_ts,0) DESC LIMIT ?`
	args = append(args, limit)

	rows, err := d.sql.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Group
	for rows.Next() {
		var g Group
		var created, updated int64
		if err := rows.Scan(&g.JID, &g.Name, &g.OwnerJID, &created, &updated, &g.IsParent, &g.LinkedParentJID); err != nil {
			return nil, err
		}
		g.CreatedAt = fromUnix(created)
		g.UpdatedAt = fromUnix(updated)
		out = append(out, g)
	}
	return out, rows.Err()
}

func (d *DB) SetAlias(jid, alias string) error {
	alias = strings.TrimSpace(alias)
	if alias == "" {
		return fmt.Errorf("alias is required")
	}
	now := time.Now().UTC().Unix()
	_, err := d.sql.Exec(`
		INSERT INTO contact_aliases(jid, alias, notes, updated_at)
		VALUES (?, ?, NULL, ?)
		ON CONFLICT(jid) DO UPDATE SET alias=excluded.alias, updated_at=excluded.updated_at
	`, jid, alias, now)
	return err
}

func (d *DB) RemoveAlias(jid string) error {
	_, err := d.sql.Exec(`DELETE FROM contact_aliases WHERE jid = ?`, jid)
	return err
}

func (d *DB) AddTag(jid, tag string) error {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return fmt.Errorf("tag is required")
	}
	now := time.Now().UTC().Unix()
	_, err := d.sql.Exec(`
		INSERT INTO contact_tags(jid, tag, updated_at) VALUES(?, ?, ?)
		ON CONFLICT(jid, tag) DO UPDATE SET updated_at=excluded.updated_at
	`, jid, tag, now)
	return err
}

func (d *DB) RemoveTag(jid, tag string) error {
	_, err := d.sql.Exec(`DELETE FROM contact_tags WHERE jid = ? AND tag = ?`, jid, tag)
	return err
}
