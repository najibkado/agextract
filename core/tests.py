from django.test import TestCase, Client
from django.urls import reverse
from core.models import Session
import os

class AgExtractFlowTest(TestCase):
    def setUp(self):
        self.client = Client()

    def test_upload_and_view(self):
        # 1. Create a dummy transcript file
        content = b"""# User
        Build me a rocket.
        
        # Agent
        Okay, here is a rocket.
        
        ```python
        print("Rocket")
        ```
        """
        
        with open('temp_test.md', 'wb') as f:
            f.write(content)
            
        # 2. Upload the file
        with open('temp_test.md', 'rb') as f:
            response = self.client.post(reverse('upload'), {'file': f})
            
        # 3. Verify Redirect
        self.assertEqual(response.status_code, 302, "Upload should redirect")
        redirect_url = response.url
        print(f"Redirected to: {redirect_url}")
        
        # 4. Verify Session Created
        session = Session.objects.last()
        self.assertIsNotNone(session)
        self.assertEqual(session.steps.count(), 2)
        
        # 5. Verify Detail Page
        response = self.client.get(redirect_url)
        self.assertEqual(response.status_code, 200)
        self.assertContains(response, "temp_test.md") 
        self.assertContains(response, "filterSteps") # Check for JS function
        self.assertContains(response, "step-role-user") # Check for step classes
        
        print("Flow test passed!")
        
        # Cleanup
        os.remove('temp_test.md')

    def test_add_tag(self):
        from core.models import Step, Session
        # Create session and step
        session = Session.objects.create(title="Test")
        step = Step.objects.create(session=session, role='user', step_type='prompt', content="Hi")
        
        # Post to add tag
        response = self.client.post(reverse('add_tag', args=[step.id]), {'tag_type': 'pivot'})
        
        self.assertEqual(response.status_code, 200)
        self.assertEqual(step.tags.count(), 1)
        self.assertEqual(step.tags.first().tag_type, 'pivot')

    def test_step_card_view(self):
        from core.models import Step, Session
        session = Session.objects.create(title="Test")
        step = Step.objects.create(session=session, role='user', step_type='prompt', content="Card Content")
        
        url = reverse('step_card', args=[step.id])
        response = self.client.get(url)
        
        self.assertEqual(response.status_code, 200)
        self.assertContains(response, "Card Content")
        self.assertContains(response, "Human") # Check for role label in card
