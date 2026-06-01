Write-Host "Seeding admin user..."
docker-compose exec python-api python /app/scripts/seed_admin.py
