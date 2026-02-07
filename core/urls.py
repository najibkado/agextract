from django.urls import path
from . import views

urlpatterns = [
    path('', views.upload_view, name='upload'),
    path('login/', views.web_login, name='web_login'),
    path('logout/', views.web_logout, name='web_logout'),
    path('dashboard/', views.dashboard, name='dashboard'),
    path('@<str:username>/', views.public_profile, name='public_profile'),
    path('session/<uuid:session_id>/', views.session_detail, name='session_detail'),
    path('step/<int:step_id>/tag/', views.add_tag, name='add_tag'),
    path('step/<int:step_id>/card/', views.step_card, name='step_card'),
]
