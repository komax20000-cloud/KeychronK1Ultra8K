@echo off
rem Builds dist\keychron_flasher.exe (run on Windows with Python 3.12)
cd /d "%~dp0"
python -m pip install -r requirements.txt || exit /b 1
python test_protocol.py 2>nul
python -m PyInstaller --onefile --console --name keychron_flasher keychron_flasher.py || exit /b 1
echo Built: %~dp0dist\keychron_flasher.exe
