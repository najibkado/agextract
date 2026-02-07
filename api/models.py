import secrets
from django.conf import settings
from django.db import models
from django.utils import timezone


def generate_access_token():
    return 'agx_' + secrets.token_hex(20)


def generate_refresh_token():
    return secrets.token_hex(32)


def generate_oauth_code():
    return secrets.token_hex(32)


class APIToken(models.Model):
    user = models.ForeignKey(
        settings.AUTH_USER_MODEL, on_delete=models.CASCADE,
        related_name='api_tokens',
    )
    access_token = models.CharField(
        max_length=64, unique=True, default=generate_access_token,
    )
    refresh_token = models.CharField(
        max_length=64, unique=True, default=generate_refresh_token,
    )
    created_at = models.DateTimeField(auto_now_add=True)
    expires_at = models.DateTimeField()
    revoked = models.BooleanField(default=False)

    def is_valid(self):
        return not self.revoked and self.expires_at > timezone.now()

    def __str__(self):
        return f"Token for {self.user.username} ({'valid' if self.is_valid() else 'expired/revoked'})"


class OAuthCode(models.Model):
    user = models.ForeignKey(
        settings.AUTH_USER_MODEL, on_delete=models.CASCADE,
        related_name='oauth_codes',
    )
    code = models.CharField(max_length=64, unique=True, default=generate_oauth_code)
    redirect_uri = models.CharField(max_length=500)
    state = models.CharField(max_length=128, blank=True, default='')
    created_at = models.DateTimeField(auto_now_add=True)
    used = models.BooleanField(default=False)

    def is_valid(self):
        """Codes expire after 5 minutes."""
        if self.used:
            return False
        age = (timezone.now() - self.created_at).total_seconds()
        return age < 300

    def __str__(self):
        return f"OAuth code for {self.user.username} ({'used' if self.used else 'active'})"
