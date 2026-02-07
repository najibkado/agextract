import functools
from django.http import JsonResponse
from .models import APIToken


def get_token_from_request(request):
    """Extract Bearer token from Authorization header."""
    auth_header = request.META.get('HTTP_AUTHORIZATION', '')
    if auth_header.startswith('Bearer '):
        return auth_header[7:]
    return None


def require_api_auth(view_func):
    """Decorator that validates Bearer token and sets request.api_user."""
    @functools.wraps(view_func)
    def wrapper(request, *args, **kwargs):
        token_str = get_token_from_request(request)
        if not token_str:
            return JsonResponse(
                {'error': 'Authentication required. Provide Bearer token.'},
                status=401,
            )
        try:
            token = APIToken.objects.select_related('user').get(
                access_token=token_str,
            )
        except APIToken.DoesNotExist:
            return JsonResponse({'error': 'Invalid token.'}, status=401)

        if not token.is_valid():
            return JsonResponse({'error': 'Token expired or revoked.'}, status=401)

        request.api_user = token.user
        request.api_token = token
        return view_func(request, *args, **kwargs)
    return wrapper
