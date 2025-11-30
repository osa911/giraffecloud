"use client";

import { useState, useEffect } from "react";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Button } from "@/components/ui/button";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog";
import { Trash, Plus, AlertTriangle, Loader2 } from "lucide-react";
import { format } from "date-fns";
import { getTokensList, revokeToken, type Token } from "@/services/api/tokenApi";
import { ApiError } from "@/services/apiClient/baseApiClient";
import TokenDialog from "./TokenDialog";
import { toast } from "@/lib/toast";

export default function TokenManagement() {
  const [tokens, setTokens] = useState<Token[]>([]);
  const [open, setOpen] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  const fetchTokens = async () => {
    try {
      setLoading(true);
      const data = await getTokensList();
      setTokens(data);
      setError(null);
    } catch (err) {
      const errorMessage = err instanceof ApiError ? err.message : "Failed to fetch tokens";
      setError(errorMessage);
      console.error("Error fetching tokens:", err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchTokens();
  }, []);

  const handleTokenCreated = () => {
    fetchTokens();
  };

  const handleRevokeToken = async (id: string) => {

    try {
      await revokeToken(id);
      setTokens(tokens.filter((token) => token.id !== id));
      toast.success("Token revoked successfully");
    } catch (err) {
      const errorMessage = err instanceof ApiError ? err.message : "Failed to revoke token";
      toast.error(errorMessage);
      console.error("Error revoking token:", err);
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h3 className="text-lg font-medium">API Tokens</h3>
          <p className="text-sm text-muted-foreground">
            Manage your API tokens for accessing the GiraffeCloud API programmatically.
          </p>
        </div>
        <Button onClick={() => setOpen(true)}>
          <Plus className="mr-2 h-4 w-4" /> Create New Token
        </Button>
      </div>

      {error && (
        <Alert variant="destructive">
          <AlertTriangle className="h-4 w-4" />
          <AlertTitle>Error</AlertTitle>
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}

      <div className="rounded-md border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Name</TableHead>
              <TableHead>Created</TableHead>
              <TableHead>Last Used</TableHead>
              <TableHead>Expires</TableHead>
              <TableHead className="text-right">Actions</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {loading ? (
              <TableRow>
                <TableCell colSpan={5} className="h-24 text-center">
                  <div className="flex items-center justify-center gap-2 text-muted-foreground">
                    <Loader2 className="h-4 w-4 animate-spin" />
                    Loading tokens...
                  </div>
                </TableCell>
              </TableRow>
            ) : tokens.length === 0 ? (
              <TableRow>
                <TableCell colSpan={5} className="h-24 text-center text-muted-foreground">
                  No tokens found. Create one to get started.
                </TableCell>
              </TableRow>
            ) : (
              tokens.map((token) => (
                <TableRow key={token.id}>
                  <TableCell className="font-medium">{token.name}</TableCell>
                  <TableCell>{format(new Date(token.created_at), "PPp")}</TableCell>
                  <TableCell>
                    {token.last_used_at
                      ? format(new Date(token.last_used_at), "PPp")
                      : "Never"}
                  </TableCell>
                  <TableCell>
                    {token.expires_at
                      ? format(new Date(token.expires_at), "PPp")
                      : "Never"}
                  </TableCell>
                  <TableCell className="text-right">
                    <AlertDialog>
                      <AlertDialogTrigger asChild>
                        <Button
                          variant="ghost"
                          size="icon"
                          className="text-destructive hover:text-destructive hover:bg-destructive/10"
                        >
                          <Trash className="h-4 w-4" />
                          <span className="sr-only">Revoke</span>
                        </Button>
                      </AlertDialogTrigger>
                      <AlertDialogContent>
                        <AlertDialogHeader>
                          <AlertDialogTitle>Are you absolutely sure?</AlertDialogTitle>
                          <AlertDialogDescription>
                            This action cannot be undone. This will permanently revoke the token
                            and any applications using it will stop working immediately.
                          </AlertDialogDescription>
                        </AlertDialogHeader>
                        <AlertDialogFooter>
                          <AlertDialogCancel>Cancel</AlertDialogCancel>
                          <AlertDialogAction
                            onClick={() => handleRevokeToken(token.id)}
                            className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
                          >
                            Revoke Token
                          </AlertDialogAction>
                        </AlertDialogFooter>
                      </AlertDialogContent>
                    </AlertDialog>
                  </TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </div>

      <TokenDialog
        open={open}
        onOpenChange={setOpen}
        onSuccess={handleTokenCreated}
      />
    </div>
  );
}
