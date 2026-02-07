from django import forms

class UploadSessionForm(forms.Form):
    file = forms.FileField(
        label='Select your transcript (.md)',
        help_text='Upload a Markdown export from Cursor or Claude',
        widget=forms.ClearableFileInput(attrs={
            'class': 'block w-full text-sm text-gray-300 file:mr-4 file:py-2 file:px-4 file:rounded-full file:border-0 file:text-sm file:font-semibold file:bg-brand-accent file:text-white hover:file:bg-sky-600 cursor-pointer',
        })
    )
