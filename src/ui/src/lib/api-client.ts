import createClient from "openapi-fetch";

import type { paths } from "./api";

const $api = createClient<paths>({
  baseUrl: "http://localhost:8765",
  headers: {
    'Content-Type': 'application/json',
    'X-API-Key': 'bg-sk-Uid-1234567890',
  },
});

export { $api }
