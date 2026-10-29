@echo off
title Central de Candidaturas
cd /d "%~dp0"
where node >nul 2>nul || (echo Instale o Node.js 18 ou mais novo: https://nodejs.org & pause & exit /b 1)
set CENTRAL_ABRIR=1
node servidor\index.mjs
pause
