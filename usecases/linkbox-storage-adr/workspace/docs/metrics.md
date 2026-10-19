# Usage, October 2026

From the platform dashboard, 1 to 14 October 2026.

- About 1,900 users have an API key; about 600 use linkbox in a given week.
- Requests: 4 to 6 per second at peak, about 95% of them reads (`GET /links` and `GET /links/{id}`).
- New links: about 3,500 a day.
- p99 latency: 4 ms. Product's target is a p99 under 100 ms for every request.
- Product expects the number of users to triple within a year.
