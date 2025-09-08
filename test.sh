#!/bin/bash

echo "🔍 Validating GPT-OSS Chat Agent Implementation"
echo "=============================================="


mkdir -p quick_test
cd quick_test

../coder "Create a simple Go HTTP server with the following features: \
- Listens on port 8080 \
- Has a /hello endpoint that returns 'Hello, World!' \
- Logs each request to the console \
- Gracefully handles shutdown on SIGINT and SIGTERM signals \
-- Additionally, it should have the following crud endpoints that save and return json data from a database.json file: \
- /users: Returns a list of users in JSON format, where each user has an id, name, and email. The list should contain at least 3 users. \
- /user/{id}: Returns the details of a user with the specified id. If the user does not exist, return a 404 status code. \
- /add_user: Accepts a POST request with a JSON body containing name and email fields
- /update_user/{id}: Accepts a PUT request to update the name and/or email of the user with the specified id
- /delete_user/{id}: Accepts a DELETE request to remove the user with the specified id from the list
- add a curl test script to test all the endpoints and their functionality and use it as a health check and e2e test for the server \
- ensure the server can handle concurrent requests and includes basic error handling for invalid input and server errors
"
