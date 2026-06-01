# REST Contract: Go Escrow Service ↔ Blockchain Simulator

## Base URL
| Environment | URL |
|---|---|
| Docker | `http://blockchain-sim:8082` |
| Local | `http://localhost:8082` |

## Common Headers
| Header | Required | Description |
|---|---|---|
| `Content-Type` | Yes | `application/json` |
| `X-Request-ID` | No | UUID for request tracing |

## Endpoints

### POST /v1/chain/submit — Submit Transaction Event
Called by Go Escrow after each valid state-machine transition.

**Request:**
```json
{
  "order_id": "uuid",
  "action": "CREATED|FUNDED|IN_PROGRESS|COMPLETED|RELEASED|CANCELLED|DISPUTED",
  "data": {
    "escrow_id": "uuid",
    "status": "CREATED|FUNDED|IN_PROGRESS|COMPLETED|RELEASED|CANCELLED|DISPUTED",
    "amount": "100.0000"
  }
}
```

**Responses:**
| Code | Description |
|---|---|
| `201` | Created — block appended to chain |
| `400` | Chain integrity breached |
| `422` | Validation error |

**Success Response (201):**
```json
{
  "block_index": 1,
  "tx_hash": "abcd1234...",
  "prev_hash": "0000..."
}
```

## Event Types (action field)
| Action | Trigger | Description |
|---|---|---|
| `CREATED` | Escrow account created | New escrow account opened |
| `FUNDED` | Escrow funded | Funds deposited into escrow |
| `IN_PROGRESS` | Escrow advanced to in_progress | Service delivery started |
| `COMPLETED` | Escrow advanced to completed | Service delivery completed |
| `RELEASED` | Escrow released | Funds released to provider |
| `CANCELLED` | Escrow cancelled | Funds returned to buyer |
| `DISPUTED` | Dispute opened | Dispute raised on escrow |

## Retry Policy
- Max retries: 3
- Backoff: 100ms base + jitter (±50ms)
- On failure: log warning, queue for retry, do not block main flow
