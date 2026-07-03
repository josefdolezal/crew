package store

import (
	"database/sql"
	"errors"
	"time"

	"github.com/josefdolezal/crew/internal/proto"
)

func (s *Store) InsertMessage(m proto.Message) (int64, error) {
	res, err := s.db.Exec(
		`INSERT INTO messages (sender, recipient, kind, status, body, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		m.Sender, m.Recipient, m.Kind, m.Status, m.Body, m.CreatedAt.Unix(),
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// Inbox returns messages for a recipient, oldest first. With unreadOnly,
// read messages are skipped. With drain, returned messages are marked read.
func (s *Store) Inbox(recipient string, unreadOnly, drain bool) ([]proto.Message, error) {
	q := `SELECT id, sender, recipient, kind, status, body, created_at, read_at
	      FROM messages WHERE recipient = ?`
	if unreadOnly {
		q += ` AND read_at IS NULL`
	}
	q += ` ORDER BY id`
	rows, err := s.db.Query(q, recipient)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var msgs []proto.Message
	for rows.Next() {
		m, err := scanMessage(rows)
		if err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if drain && len(msgs) > 0 {
		now := time.Now().Unix()
		for i := range msgs {
			if msgs[i].ReadAt != nil {
				continue
			}
			if _, err := s.db.Exec(`UPDATE messages SET read_at = ? WHERE id = ?`, now, msgs[i].ID); err != nil {
				return nil, err
			}
			t := time.Unix(now, 0)
			msgs[i].ReadAt = &t
		}
	}
	return msgs, nil
}

// NextUnreadReport returns the oldest unconsumed report from an agent,
// or ErrNotFound. Reports are consumed (marked read) when a wait returns
// them or the inbox is drained, giving `crew wait` round semantics: one
// report unblocks exactly one wait, in order.
func (s *Store) NextUnreadReport(sender string) (proto.Message, error) {
	row := s.db.QueryRow(
		`SELECT id, sender, recipient, kind, status, body, created_at, read_at
		 FROM messages WHERE sender = ? AND kind = 'report' AND read_at IS NULL
		 ORDER BY id LIMIT 1`, sender)
	m, err := scanMessage(row)
	if errors.Is(err, sql.ErrNoRows) {
		return m, ErrNotFound
	}
	return m, err
}

// MarkRead consumes a single message.
func (s *Store) MarkRead(id int64) error {
	_, err := s.db.Exec(`UPDATE messages SET read_at = ? WHERE id = ? AND read_at IS NULL`, time.Now().Unix(), id)
	return err
}

// DeleteMessagesFrom removes an agent's outgoing reports so a re-spawned
// agent with the same name doesn't inherit a stale "done".
func (s *Store) DeleteMessagesFrom(sender string) error {
	_, err := s.db.Exec(`DELETE FROM messages WHERE sender = ? AND kind = 'report'`, sender)
	return err
}

func scanMessage(row scanner) (proto.Message, error) {
	var m proto.Message
	var createdAt int64
	var readAt sql.NullInt64
	err := row.Scan(&m.ID, &m.Sender, &m.Recipient, &m.Kind, &m.Status, &m.Body, &createdAt, &readAt)
	if err != nil {
		return m, err
	}
	m.CreatedAt = time.Unix(createdAt, 0)
	if readAt.Valid {
		t := time.Unix(readAt.Int64, 0)
		m.ReadAt = &t
	}
	return m, nil
}
