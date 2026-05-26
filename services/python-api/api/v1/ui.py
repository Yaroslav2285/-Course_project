# LR #6: Web/DB — UI router serving Jinja2 templates + static files
# LR #2: Modern Python — async paths, Path utilities
from pathlib import Path

from fastapi import APIRouter, Request
from fastapi.responses import HTMLResponse
from fastapi.staticfiles import StaticFiles
from fastapi.templating import Jinja2Templates

router = APIRouter()

BASE_DIR = Path(__file__).resolve().parent.parent.parent
TEMPLATES_DIR = BASE_DIR / "templates"
STATIC_DIR = BASE_DIR / "static"

templates = Jinja2Templates(directory=str(TEMPLATES_DIR))


@router.get("/", response_class=HTMLResponse, include_in_schema=False)
async def index_page(request: Request):
    return templates.TemplateResponse(request, "index.html", {"request": request})


@router.get("/login", response_class=HTMLResponse, include_in_schema=False)
async def login_page(request: Request):
    return templates.TemplateResponse(request, "login.html", {"request": request})


@router.get("/dashboard", response_class=HTMLResponse, include_in_schema=False)
async def dashboard_page(request: Request):
    return templates.TemplateResponse(request, "dashboard.html", {"request": request})


@router.get("/escrow/{order_id}", response_class=HTMLResponse, include_in_schema=False)
async def escrow_page(request: Request, order_id: str):
    return templates.TemplateResponse(
        request, "escrow_status.html", {"request": request, "order_id": order_id}
    )


def mount_static(app):
    if Path(STATIC_DIR).exists():
        app.mount("/static", StaticFiles(directory=str(STATIC_DIR)), name="static")
