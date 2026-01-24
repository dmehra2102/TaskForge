-- Drop triggers
DROP TRIGGER IF EXISTS log_todo_status_change ON todos;
DROP TRIGGER IF EXISTS update_todos_updated_at ON todos;

-- Drop functions
DROP FUNCTION IF EXISTS log_status_change();
DROP FUNCTION IF EXISTS update_updated_at_column();

-- Drop tables
DROP TABLE IF EXISTS todo_audit;
DROP TABLE IF EXISTS todos;

-- Drop Indexes
DROP INDEX IF EXISTS idx_todos_tenant_id;
DROP INDEX IF EXISTS idx_todos_owner_id;
DROP INDEX IF EXISTS idx_todos_assigned_to;
DROP INDEX IF EXISTS idx_todos_status;
DROP INDEX IF EXISTS idx_todos_priority;
DROP INDEX IF EXISTS idx_todos_due_date;
DROP INDEX IF EXISTS idx_todos_created_at;
DROP INDEX IF EXISTS idx_todos_updated_at;

DROP INDEX IF EXISTS idx_todos_tags;
DROP INDEX IF EXISTS idx_todos_search;

DROP INDEX IF EXISTS idx_todo_audit_todo_id;