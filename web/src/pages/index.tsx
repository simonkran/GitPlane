import { useEffect } from "react";
import { useRouter } from "next/router";

export default function IndexPage() {
  const router = useRouter();

  useEffect(() => {
    const token = localStorage.getItem("token");
    if (token) {
      router.replace("/clusters");
    } else {
      router.replace("/login");
    }
  }, [router]);

  return null;
}
