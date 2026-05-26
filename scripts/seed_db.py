# LR #3: OOP/FP
# LR #4: Async/Web
# Script for seeding demo data via API

import sys
import uuid

import httpx

BASE_URL = sys.argv[1] if len(sys.argv) > 1 else "http://localhost:8000"
client = httpx.Client(base_url=BASE_URL, follow_redirects=True)


def seed():
    print(f"Seeding {BASE_URL} ...")

    r = client.post("/v1/auth/register", json={
        "email": "client@demo.com",
        "password": "Demo123!",
    })
    if r.status_code == 409:
        print("  client@demo.com already exists, logging in")
        r = client.post("/v1/auth/login", json={
            "email": "client@demo.com",
            "password": "Demo123!",
        })
    client_token = r.json()["data"]["access_token"]
    client_id = r.json()["data"]["user"]["id"]
    print(f"  Client: {client_id}")

    r = client.post("/v1/auth/register", json={
        "email": "provider@demo.com",
        "password": "Demo123!",
    })
    if r.status_code == 409:
        print("  provider@demo.com already exists, logging in")
        r = client.post("/v1/auth/login", json={
            "email": "provider@demo.com",
            "password": "Demo123!",
        })
    provider_token = r.json()["data"]["access_token"]
    provider_id = r.json()["data"]["user"]["id"]
    print(f"  Provider: {provider_id}")

    r = client.post(
        "/v1/services/",
        headers={"Authorization": f"Bearer {provider_token}"},
        json={
            "title": "Web Development",
            "description": "Full-stack web development service",
            "price": "500.0000",
        },
    )
    service = r.json()["data"]
    service_id = service["id"]
    print(f"  Service: {service_id} (status: {service['status']})")

    r = client.put(
        f"/v1/services/{service_id}",
        headers={"Authorization": f"Bearer {provider_token}"},
        json={"status": "published"},
    )
    print(f"  Service published: {r.status_code}")

    r = client.post(
        "/v1/orders/",
        headers={"Authorization": f"Bearer {client_token}"},
        json={
            "buyer_id": client_id,
            "seller_id": provider_id,
            "service_id": service_id,
            "amount": "500.0000",
        },
    )
    order = r.json()["data"]
    order_id = order["id"]
    print(f"  Order: {order_id} (status: {order['status']})")

    print(f"\nDemo data seeded successfully!")
    print(f"  Client:    client@demo.com / Demo123!")
    print(f"  Provider:  provider@demo.com / Demo123!")
    print(f"  Service:   {service_id}")
    print(f"  Order:     {order_id}")


if __name__ == "__main__":
    seed()
