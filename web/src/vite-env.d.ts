/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_DEV_AUTH?: string;
  readonly DEV: boolean;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
