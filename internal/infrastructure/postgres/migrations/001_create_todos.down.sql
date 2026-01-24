CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE IF NOT EXISTS todos (
    id UUID PRIMARY KEY,
    title VARCHAR(200) NOT NULL,
    description TEXT,
    status INTEGER NOT NULL DEFAULT 1,
    priority INTEGER NOT NULL DEFAULT 2,
    due_date TIMESTAMP WITH TIME ZONE,
    tags TEXT[] DEFAULT {},
    owner_id VARCHAR(100) NOT NULL,
    assigned_to VARCHAR(100),
    tenant_id VARCHAR(100) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE,
    version BIGINT NOT NULL DEFAULT 1,

    CONSTRAINT title_not_empty CHECK (LENGTH(title) > 0),
    CONSTRAINT valid_status CHECK(status BETWEEN 1 AND 4),
    CONSTRAINT valid_property CHECK (priority BETWEEN 1 AND 4)
);

-- INDEXES for common Query Patterns
CREATE INDEX idx_todos_tenant_id ON todos(tenant_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_todos_owner_id ON todos(owner_id, tenant_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_todos_assigned_to ON todos(assigned_to, tenant_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_todos_status ON todos(status, tenant_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_todos_priority ON todos(priority, tenant_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_todos_due_date ON todos(due_date, tenant_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_todos_created_at ON todos(created_at DESC, tenant_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_todos_updated_at ON todos(updated_at DESC, tenant_id) WHERE deleted_at IS NULL;

CREATE INDEX idx_todos_tags ON todos USING GIN(tags) WHERE deleted_at IS NULL;

-- Full-Text Search Index
CREATE INDEX idx_todos_search ON todos USING GIN(
    to_tsvector('english', COALESCE(title, '') || ' ' || COALESCE(description, ''))
) WHERE deleted_at IS NULL;

-- Create audit table for tracking changes
CREATE TABLE IF NOT EXISTS todo_audit (
    id BIGSERIAL PRIMARY KEY,
    todo_id UUID NOT NULL,
    action VARCHAR(20) NOT NULL,
    old_status INTEGER,
    new_status INTEGER,
    changed_by VARCHAR(100) NOT NULL,
    changed_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    reason TEXT,
    metadata JSONB
);

CREATE INDEX idx_todo_audit_todo_id ON todo_audit(todo_id, changed_at DESC);

-- function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;


-- Trigger to automatically update updated_at
CREATE TRIGGER update_todos_updated_at
    BEFORE UPDATE ON todos
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();


-- Function to log status changes
CREATE OR REPLACE FUNCTION log_status_change()
RETURN TRIGGER AS $$
BEGIN
    IF OLD.status IS DISTINCT FROM NEW.status THEN
        INSERT INTO todo_audit (
            todo_id,
            action,
            old_status,
            new_status,
            changed_by,
            metadata
        ) VALUES (
            NEW.id,
            'STATUS_CHANGE',
            OLD.status,
            NEW.status,
            COALESCE(current_setting('app.current_user_id', true), 'system'),
            jsonb_build_object(
                'old_version', OLD.version,
                'new_version', NEW.version
            )
        )
END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- trigger for audit logging
CREATE TRIGGER log_todo_status_change
    AFTER UPDATE ON todos
    FOR EACH ROW
    WHEN (OLD.status IS DISTINCT FROM NEW.status)
    EXECUTE FUNCTION log_status_change();