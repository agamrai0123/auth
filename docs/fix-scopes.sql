-- Fix endpoint scopes to match client capabilities
-- test-client has scopes: ["read:ltp", "read:quote"]
-- Update endpoints to use scopes that test-client actually has

UPDATE endpoints 
SET scope = 'read:ltp' 
WHERE client_id = 'test-client' AND endpoint_url = 'http://localhost:8082/resource1';

UPDATE endpoints 
SET scope = 'read:quote' 
WHERE client_id = 'test-client' AND endpoint_url = 'http://localhost:8082/resource2';

COMMIT;

-- Verify the changes
SELECT client_id, scope, method, endpoint_url FROM endpoints 
WHERE client_id = 'test-client' 
ORDER BY endpoint_url;
