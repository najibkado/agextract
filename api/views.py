import hashlib
import json
from datetime import timedelta
from string import Template

from django.contrib.auth import authenticate, login
from django.http import JsonResponse, HttpResponse
from django.shortcuts import redirect
from django.utils import timezone
from django.views.decorators.csrf import csrf_exempt
from django.views.decorators.http import require_GET, require_POST

from core.models import Session, Step
from core.parser import TranscriptParser

from .auth import require_api_auth, get_token_from_request
from .models import APIToken, OAuthCode


# ---------------------------------------------------------------------------
# OAuth Endpoints
# ---------------------------------------------------------------------------

LOGIN_TEMPLATE = Template("""<!DOCTYPE html>
<html>
<head><title>agextract — Login</title>
<style>
  body { font-family: system-ui; background: #0f172a; color: #e2e8f0; display: flex; justify-content: center; align-items: center; min-height: 100vh; margin: 0; }
  .card { background: #1e293b; padding: 2rem; border-radius: 12px; width: 320px; }
  h2 { margin-top: 0; }
  label { display: block; margin-top: 1rem; font-size: 0.875rem; color: #94a3b8; }
  input { width: 100%; padding: 0.5rem; margin-top: 0.25rem; border: 1px solid #334155; border-radius: 6px; background: #0f172a; color: #e2e8f0; box-sizing: border-box; }
  button { margin-top: 1.5rem; width: 100%; padding: 0.6rem; background: #3b82f6; color: white; border: none; border-radius: 6px; cursor: pointer; font-size: 1rem; }
  button:hover { background: #2563eb; }
  .error { color: #f87171; font-size: 0.875rem; margin-top: 0.5rem; }
</style></head>
<body>
<div class="card">
  <h2>ag<strong>extract</strong></h2>
  <p style="color:#94a3b8;font-size:0.875rem;">Sign in to authorize the CLI</p>
  $error
  <form method="post">
    <label>Username<input name="username" autofocus required></label>
    <label>Password<input name="password" type="password" required></label>
    <input type="hidden" name="next" value="$next_url">
    <button type="submit">Sign In</button>
  </form>
</div>
</body></html>""")


@csrf_exempt
def oauth_authorize(request):
    """
    GET/POST /api/v1/oauth/authorize/?redirect_uri=...&state=...
    Shows login form if not authenticated, then redirects with OAuth code.
    """
    redirect_uri = request.GET.get('redirect_uri', '') or request.POST.get('redirect_uri', '')
    state = request.GET.get('state', '') or request.POST.get('state', '')

    # Preserve query params for the next URL after login
    next_url = request.get_full_path()

    # Handle login POST
    if request.method == 'POST' and not request.user.is_authenticated:
        username = request.POST.get('username', '')
        password = request.POST.get('password', '')
        user = authenticate(request, username=username, password=password)
        if user is not None:
            login(request, user)
        else:
            html = LOGIN_TEMPLATE.substitute(
                error='<p class="error">Invalid username or password.</p>',
                next_url=request.POST.get('next', next_url),
            )
            return HttpResponse(html, status=200)

    # Show login form if not authenticated
    if not request.user.is_authenticated:
        html = LOGIN_TEMPLATE.substitute(error='', next_url=next_url)
        return HttpResponse(html, status=200)

    # User is authenticated — issue code
    if not redirect_uri:
        return JsonResponse({'error': 'redirect_uri is required'}, status=400)

    code = OAuthCode.objects.create(
        user=request.user,
        redirect_uri=redirect_uri,
        state=state,
    )

    separator = '&' if '?' in redirect_uri else '?'
    callback_url = f"{redirect_uri}{separator}code={code.code}&state={state}"
    return redirect(callback_url)


@csrf_exempt
@require_POST
def oauth_token(request):
    """
    POST /api/v1/oauth/token/
    Exchange an authorization code for access + refresh tokens,
    or refresh an existing token.
    """
    try:
        body = json.loads(request.body)
    except (json.JSONDecodeError, ValueError):
        return JsonResponse({'error': 'Invalid JSON body'}, status=400)

    grant_type = body.get('grant_type', 'authorization_code')

    if grant_type == 'authorization_code':
        code_str = body.get('code', '')
        if not code_str:
            return JsonResponse({'error': 'code is required'}, status=400)

        try:
            oauth_code = OAuthCode.objects.select_related('user').get(code=code_str)
        except OAuthCode.DoesNotExist:
            return JsonResponse({'error': 'Invalid code'}, status=400)

        if not oauth_code.is_valid():
            return JsonResponse({'error': 'Code expired or already used'}, status=400)

        oauth_code.used = True
        oauth_code.save()

        token = APIToken.objects.create(
            user=oauth_code.user,
            expires_at=timezone.now() + timedelta(days=30),
        )

        return JsonResponse({
            'access_token': token.access_token,
            'refresh_token': token.refresh_token,
            'token_type': 'Bearer',
            'expires_in': 30 * 24 * 3600,
        })

    elif grant_type == 'refresh_token':
        refresh = body.get('refresh_token', '')
        if not refresh:
            return JsonResponse({'error': 'refresh_token is required'}, status=400)

        try:
            old_token = APIToken.objects.select_related('user').get(
                refresh_token=refresh,
            )
        except APIToken.DoesNotExist:
            return JsonResponse({'error': 'Invalid refresh token'}, status=400)

        if old_token.revoked:
            return JsonResponse({'error': 'Token has been revoked'}, status=400)

        # Revoke old token, issue new one
        old_token.revoked = True
        old_token.save()

        new_token = APIToken.objects.create(
            user=old_token.user,
            expires_at=timezone.now() + timedelta(days=30),
        )

        return JsonResponse({
            'access_token': new_token.access_token,
            'refresh_token': new_token.refresh_token,
            'token_type': 'Bearer',
            'expires_in': 30 * 24 * 3600,
        })

    return JsonResponse({'error': 'Unsupported grant_type'}, status=400)


@csrf_exempt
@require_POST
@require_api_auth
def oauth_revoke(request):
    """POST /api/v1/oauth/revoke/ — revoke the current token."""
    request.api_token.revoked = True
    request.api_token.save()
    return JsonResponse({'status': 'revoked'})


# ---------------------------------------------------------------------------
# User Info
# ---------------------------------------------------------------------------

@require_GET
@require_api_auth
def me(request):
    """GET /api/v1/me/ — current user info."""
    user = request.api_user
    return JsonResponse({
        'id': user.id,
        'username': user.username,
        'email': user.email,
    })


# ---------------------------------------------------------------------------
# Session Endpoints
# ---------------------------------------------------------------------------

@csrf_exempt
@require_POST
@require_api_auth
def session_create(request):
    """
    POST /api/v1/sessions/
    Create a session from structured JSON (pre-parsed by CLI).
    Idempotent on source + source_session_id per user.
    """
    try:
        body = json.loads(request.body)
    except (json.JSONDecodeError, ValueError):
        return JsonResponse({'error': 'Invalid JSON body'}, status=400)

    source = body.get('source', 'upload')
    source_session_id = body.get('source_session_id', '')

    # Idempotency: return existing session if same source + source_session_id
    if source_session_id:
        existing = Session.objects.filter(
            user=request.api_user,
            source=source,
            source_session_id=source_session_id,
        ).first()
        if existing:
            return _session_to_json(existing, status=200)

    # Content hash dedup: hash the JSON body for structured uploads
    content_hash = hashlib.sha256(request.body).hexdigest()
    existing = Session.objects.filter(
        user=request.api_user,
        content_hash=content_hash,
    ).first()
    if existing:
        return _session_to_json(existing, status=200)

    session = Session.objects.create(
        user=request.api_user,
        title=body.get('title', 'Untitled Session'),
        source=source,
        source_session_id=source_session_id,
        content_hash=content_hash,
        duration_seconds=body.get('duration_seconds'),
        token_usage=body.get('token_usage'),
        file_count=body.get('file_count'),
    )

    # Create steps
    for step_data in body.get('steps', []):
        Step.objects.create(
            session=session,
            role=step_data.get('role', 'user'),
            step_type=step_data.get('step_type', 'text'),
            content=step_data.get('content', ''),
            order=step_data.get('order', 0),
        )

    return _session_to_json(session, status=201)


@csrf_exempt
@require_POST
@require_api_auth
def session_upload(request):
    """
    POST /api/v1/sessions/upload/
    Upload a raw .md/.jsonl file for server-side parsing.
    Deduplicates on content_hash per user.
    """
    uploaded_file = request.FILES.get('file')
    if not uploaded_file:
        return JsonResponse({'error': 'No file provided'}, status=400)

    content = uploaded_file.read()
    content_hash = hashlib.sha256(content).hexdigest()

    # Dedup: return existing session if same content was already uploaded by this user
    existing = Session.objects.filter(
        user=request.api_user,
        content_hash=content_hash,
    ).first()
    if existing:
        return _session_to_json(existing, status=200)

    title = request.POST.get('title', uploaded_file.name)

    parser = TranscriptParser(content)
    session = parser.parse(title=title)

    # Attach user, source info, and content hash
    session.user = request.api_user
    session.source = request.POST.get('source', 'upload')
    session.source_session_id = request.POST.get('source_session_id', '')
    session.content_hash = content_hash
    session.save()

    return _session_to_json(session, status=201)


@require_GET
@require_api_auth
def session_detail(request, session_id):
    """GET /api/v1/sessions/<uuid>/ — retrieve session + steps as JSON."""
    try:
        session = Session.objects.get(id=session_id, user=request.api_user)
    except Session.DoesNotExist:
        return JsonResponse({'error': 'Session not found'}, status=404)

    return _session_to_json(session)


def _session_to_json(session, status=200):
    """Helper to serialize a Session with its steps."""
    steps = list(session.steps.all().values(
        'id', 'role', 'step_type', 'content', 'order', 'timestamp',
    ))
    return JsonResponse({
        'id': str(session.id),
        'title': session.title,
        'source': session.source,
        'source_session_id': session.source_session_id,
        'uploaded_at': session.uploaded_at.isoformat(),
        'duration_seconds': session.duration_seconds,
        'token_usage': session.token_usage,
        'file_count': session.file_count,
        'steps': steps,
    }, status=status)
