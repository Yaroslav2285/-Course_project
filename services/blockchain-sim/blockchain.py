import hashlib
import json
from datetime import datetime, timezone


class Block:
    # LR #14: Структура блока цепочки — index, timestamp, prev_hash, transactions, nonce, hash
    def __init__(
        self,
        index: int,
        timestamp: str,
        prev_hash: str,
        transactions: list,
        nonce: int = 0,
    ):
        self.index = index
        self.timestamp = timestamp
        self.prev_hash = prev_hash
        self.transactions = transactions
        self.nonce = nonce
        self.hash = self._calculate_hash()

    # LR #14: SHA-256 хеширование для обеспечения целостности данных
    def _calculate_hash(self) -> str:
        raw = f"{self.index}{self.timestamp}{self.prev_hash}{json.dumps(self.transactions, sort_keys=True, default=str)}{self.nonce}"
        return hashlib.sha256(raw.encode()).hexdigest()

    def to_dict(self) -> dict:
        return {
            "index": self.index,
            "timestamp": self.timestamp,
            "prev_hash": self.prev_hash,
            "transactions": self.transactions,
            "nonce": self.nonce,
            "hash": self.hash,
        }

    @classmethod
    def from_dict(cls, data: dict) -> "Block":
        block = cls(
            index=data["index"],
            timestamp=data["timestamp"],
            prev_hash=data["prev_hash"],
            transactions=data["transactions"],
            nonce=data.get("nonce", 0),
        )
        block.hash = data["hash"]
        return block


class Blockchain:
    # LR #14: Иммутабельная цепочка блоков с валидацией связности
    def __init__(self):
        self.chain: list[Block] = []
        self._init_genesis()

    def _init_genesis(self):
        genesis = Block(
            index=0,
            timestamp=datetime.now(timezone.utc).isoformat(),
            prev_hash="0" * 64,
            transactions=[{"genesis": True}],
            nonce=0,
        )
        self.chain.append(genesis)

    def add_block(self, tx_data: dict) -> dict:
        prev_block = self.chain[-1]
        new_block = Block(
            index=prev_block.index + 1,
            timestamp=datetime.now(timezone.utc).isoformat(),
            prev_hash=prev_block.hash,
            transactions=[tx_data],
        )
        self.chain.append(new_block)
        return {
            "block_index": new_block.index,
            "tx_hash": new_block.hash,
            "prev_hash": new_block.prev_hash,
        }

    # LR #14: Валидация целостности всей цепочки через проверку prev_hash и пересчёт хеша
    # LR #15: Критическая проверка безопасности — обнаружение подмены данных в блоках
    def is_valid_chain(self) -> bool:
        for i in range(1, len(self.chain)):
            current = self.chain[i]
            previous = self.chain[i - 1]

            if current.prev_hash != previous.hash:
                return False

            expected_hash = current._calculate_hash()
            if current.hash != expected_hash:
                return False

        return True

    # LR #14: Поиск по order_id для построения аудит-трейла
    def get_audit_log(self, order_id: str) -> list[Block]:
        result = []
        for block in self.chain:
            for tx in block.transactions:
                if isinstance(tx, dict) and tx.get("order_id") == order_id:
                    result.append(block)
                    break
        return result

    def load_from_db(self, blocks_data: list[dict]):
        self.chain = [Block.from_dict(b) for b in blocks_data]

    def to_dict_list(self) -> list[dict]:
        return [b.to_dict() for b in self.chain]
