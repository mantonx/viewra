export interface SystemInfo {
  frontend: {
    url: string;
    port: string;
    framework: string;
  };
  backend: {
    url: string;
    port: string;
    framework: string;
  };
  database: {
    type: string;
    status: string;
  };
  environment: string;
}

export interface User {
  id: number;
  username: string;
  email: string;
  created_at: string;
}

export interface ApiResponse {
  status: number;
  data: unknown;
  error?: string;
}
