// Bentuk respons backend: auth.Profile + authPayload di internal/auth/handler.go.
export interface AuthUser {
  id: string
  username: string
  full_name: string
  email?: string | null
  is_active: boolean
  must_change_password: boolean
  roles: string[]
}

export interface AuthPayload {
  access_token: string
  token_type: string
  expires_in: number
  user: AuthUser
  permissions: string[]
}

export interface Session {
  id: string
  ip_address: string
  user_agent: string
  issued_at: string
  expires_at: string
  current: boolean
}
