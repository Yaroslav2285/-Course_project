# REST Contract: Python API ↔ Go Escrow Service

## Base URL
| Environment | URL |
|---|---|
| Docker | `http://go-escrow:8081` |
| Local | `http://localhost:8081` |

## Common Headers
| Header | Required | Description |
|---|---|---|
| `Content-Type` | Yes | `application/json` |
| `X-Request-ID` | No | UUID for request tracing (forwarded from Python API) |
| `X-Idempotency-Key` | No | UUID for idempotent POST requests |

## Unified Response Envelope
### Success
```json
{"data": {...}}
```
### Error
```json
{"errors": [{"code": "ERROR_CODE", "detail": "Human-readable message"}]}
```

## Endpoints

### POST /v1/escrow — Create Escrow Account
**Request:**
```json
{"order_id": "uuid", "amount": "100.0000"}
```
**Responses:**
| Code | Description |
|---|---|
| `201` | Created — returns `EscrowAccount` |
| `400` | `VALIDATION_ERROR` / `INVALID_UUID` / `INVALID_AMOUNT` |
| `422` | `VALIDATION_ERROR` — amount validation failed |

### POST /v1/escrow/{id}/fund — Fund Escrow
**Request:**
```json
{"amount": "100.0000"}
```
**Responses:**
| Code | Description |
|---|---|
| `200` | Success — returns updated `EscrowAccount` |
| `400` | `INVALID_UUID` / `VALIDATION_ERROR` / `INVALID_AMOUNT` |
| `404` | `NOT_FOUND` |
| `409` | `INVALID_TRANSITION` — wrong state |
| `429` | Rate-limited |

### POST /v1/escrow/{id}/release — Release Escrow
**Request:** `{}`
**Responses:**
| Code | Description |
|---|---|
| `200` | Success — returns updated `EscrowAccount` |
| `400` | `INVALID_UUID` |
| `404` | `NOT_FOUND` |
| `409` | `INVALID_TRANSITION` |

### POST /v1/escrow/{id}/dispute — Open Dispute
**Request:**
```json
{"reason": "Service not completed"}
```
**Responses:**
| Code | Description |
|---|---|
| `200` | Success — returns updated `EscrowAccount` |
| `400` | `INVALID_UUID` |
| `404` | `NOT_FOUND` |
| `409` | `INVALID_TRANSITION` |
| `422` | `VALIDATION_ERROR` — reason required |

### GET /v1/escrow/{id} — Get Escrow Details
**Responses:**
| Code | Description |
|---|---|
| `200` | Returns `EscrowAccount` with transactions |
| `400` | `INVALID_UUID` |
| `404` | `NOT_FOUND` |

## EscrowAccount Response Shape
```json
{
  "id": "uuid",
  "order_id": "uuid",
  "balance": "100.0000",
  "status": "CREATED|FUNDED|IN_PROGRESS|COMPLETED|RELEASED|DISPUTED|RESOLVED",
  "created_at": "2026-05-27T12:00:00Z",
  "updated_at": "2026-05-27T12:00:00Z"
}
```

## State Machine
```
CREATED → FUNDED → IN_PROGRESS → COMPLETED → RELEASED
                                         ↓
                                      DISPUTED → RESOLVED
```

## Error Codes
| Code | HTTP Status | Meaning |
|---|---|---|
| `VALIDATION_ERROR` | 400/422 | Invalid request body |
| `INVALID_UUID` | 400 | Malformed UUID |
| `INVALID_AMOUNT` | 400 | Non-decimal amount |
| `NOT_FOUND` | 404 | Escrow account not found |
| `INVALID_TRANSITION` | 409 | Illegal state transition |
| `RATE_LIMITED` | 429 | Too many requests (fund endpoint) |
| `INTERNAL` | 500 | Server error |
