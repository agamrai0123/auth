-- Create CLIENTS table
CREATE TABLE clients (
    client_id VARCHAR2(100) PRIMARY KEY,
    client_secret VARCHAR2(255) NOT NULL,
    client_name VARCHAR2(255),
    access_token_ttl NUMBER(10) DEFAULT 3600,
    allowed_scopes CLOB,
    created_at TIMESTAMP DEFAULT SYSTIMESTAMP,
    updated_at TIMESTAMP DEFAULT SYSTIMESTAMP,
    active NUMBER(1) DEFAULT 1 CHECK (active IN (0, 1))
);

-- Create TOKENS table
CREATE TABLE tokens (
    token_id VARCHAR2(255) PRIMARY KEY,
    token_type VARCHAR2(20) NOT NULL,
    jwt_token VARCHAR2(2000) NOT NULL,
    client_id VARCHAR2(100) NOT NULL,
    issued_at TIMESTAMP DEFAULT SYSTIMESTAMP,
    expires_at TIMESTAMP NOT NULL,
    revoked NUMBER(1) DEFAULT 0 CHECK (revoked IN (0, 1)),
    revoked_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT SYSTIMESTAMP,
    CONSTRAINT fk_tokens_client FOREIGN KEY (client_id) REFERENCES clients(client_id) ON DELETE CASCADE
);

-- Create ENDPOINTS table
CREATE TABLE endpoints (
    id NUMBER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    client_id VARCHAR2(100) NOT NULL,
    scope VARCHAR2(255) NOT NULL,
    method VARCHAR2(10) NOT NULL,
    endpoint_url VARCHAR2(500) NOT NULL,
    description VARCHAR2(500),
    active NUMBER(1) DEFAULT 1 CHECK (active IN (0, 1)),
    created_at TIMESTAMP DEFAULT SYSTIMESTAMP,
    CONSTRAINT fk_endpoints_client FOREIGN KEY (client_id) REFERENCES clients(client_id) ON DELETE CASCADE
);

-- Create indexes for performance
CREATE INDEX idx_tokens_client_id ON tokens(client_id);
CREATE INDEX idx_tokens_expires_at ON tokens(expires_at);
CREATE INDEX idx_tokens_revoked ON tokens(revoked);
CREATE INDEX idx_endpoints_client_id ON endpoints(client_id);

-- Insert sample test data
INSERT INTO clients (client_id, client_secret, client_name, access_token_ttl, allowed_scopes)
VALUES ('test-client', 'test-secret-123', 'Test Client Application', 3600, '["read:ltp", "read:quote"]');

INSERT INTO clients (client_id, client_secret, client_name, access_token_ttl, allowed_scopes)
VALUES ('test-client-2', 'secret-key-456', 'Test Client 2', 7200, '["write:ltp", "write:quote"]');

INSERT INTO endpoints (client_id, scope, method, endpoint_url, description, active)
VALUES ('test-client', 'read:orders', 'POST', 'http://localhost:8082/resource1', '', 1);

INSERT INTO endpoints (client_id, scope, method, endpoint_url, description, active)
VALUES ('test-client', 'write:orders', 'POST', 'http://localhost:8082/resource2', '', 0);

-- Commit changes
COMMIT;

-- Display table information
SELECT table_name FROM user_tables WHERE table_name IN ('CLIENTS', 'TOKENS', 'ENDPOINTS')
ORDER BY table_name;