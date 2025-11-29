"use client";

import { useActionState } from "react";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Label } from "@/components/ui/label";
import { updateProfileAction } from "@/lib/actions/user.actions";
import { User } from "@/lib/actions/user.types";
import { Loader2 } from "lucide-react";

interface ProfilePageProps {
  initialUser: User;
}

export default function ProfilePage({ initialUser }: ProfilePageProps) {
  const [state, action, loading] = useActionState(updateProfileAction, {
    name: initialUser.name,
    email: initialUser.email,
  });

  return (
    <div className="space-y-6">
      <Card>
        <CardHeader>
          <CardTitle>Profile</CardTitle>
          <CardDescription>
            Manage your public profile information.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="flex flex-col md:flex-row gap-8">
            <div className="flex flex-col items-center space-y-4">
              <Avatar className="h-24 w-24">
                <AvatarImage src={initialUser.picture} alt={state.name} />
                <AvatarFallback className="text-2xl">
                  {state.name?.charAt(0) || "U"}
                </AvatarFallback>
              </Avatar>
              <div className="text-center">
                <h3 className="font-medium">{state.name}</h3>
                <p className="text-sm text-muted-foreground">{state.email}</p>
              </div>
            </div>

            <form action={action} className="flex-1 space-y-4">
              <div className="space-y-2">
                <Label htmlFor="name">Full Name</Label>
                <Input
                  id="name"
                  name="name"
                  defaultValue={state.name}
                  required
                  disabled={loading}
                />
              </div>

              <div className="space-y-2">
                <Label htmlFor="email">Email</Label>
                <Input
                  id="email"
                  name="email"
                  defaultValue={state.email}
                  disabled
                  className="bg-muted"
                />
                <p className="text-xs text-muted-foreground">
                  Email cannot be changed.
                </p>
              </div>

              <Button type="submit" disabled={loading}>
                {loading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                {loading ? "Saving..." : "Save Changes"}
              </Button>
            </form>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
