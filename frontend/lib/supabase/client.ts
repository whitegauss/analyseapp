import { supabaseAnonKey, supabaseUrl } from "@/lib/env";
import { createBrowserClient } from "@supabase/ssr";

export function createClient() {
  return createBrowserClient(supabaseUrl(), supabaseAnonKey());
}
