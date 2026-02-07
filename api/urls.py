from django.urls import path
from . import views

app_name = 'api'

urlpatterns = [
    # OAuth
    path('oauth/authorize/', views.oauth_authorize, name='oauth_authorize'),
    path('oauth/token/', views.oauth_token, name='oauth_token'),
    path('oauth/revoke/', views.oauth_revoke, name='oauth_revoke'),

    # User
    path('me/', views.me, name='me'),

    # Sessions
    path('sessions/', views.session_create, name='session_create'),
    path('sessions/upload/', views.session_upload, name='session_upload'),
    path('sessions/<uuid:session_id>/', views.session_detail, name='session_detail'),
]
