# -*- mode: python ; coding: utf-8 -*-

import os

block_cipher = None

datas = []
for dirpath, dirnames, filenames in os.walk("./kluctl"):
    for f in filenames:
        if f.endswith(".py") or f.endswith(".pyc"):
            continue
        p = os.path.join(dirpath, f)
        datas.append((p, dirpath))
print("datas=%s" % datas)

a = Analysis(['cli.py'],
             binaries=[],
             datas=datas,
             hiddenimports=[],
             hookspath=[],
             runtime_hooks=[],
             excludes=[],
             win_no_prefer_redirects=False,
             win_private_assemblies=False,
             cipher=block_cipher,
             noarchive=False)
pyz = PYZ(a.pure, a.zipped_data,
             cipher=block_cipher)
exe = EXE(pyz,
          a.scripts,
          a.binaries,
          a.zipfiles,
          a.datas,
          [],
          name='kluctl',
          debug=False,
          bootloader_ignore_signals=False,
          strip=False,
          upx=True,
          upx_exclude=[],
          runtime_tmpdir=None,
          console=True )
