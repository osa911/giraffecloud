"use client";

import { useState, useEffect } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import * as z from "zod";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
  FormDescription,
} from "@/components/ui/form";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Loader2, Wand2, Globe, AlertTriangle } from "lucide-react";
import { toast } from "@/lib/toast";
import type { Tunnel, TunnelCreateResponse } from "@/types/tunnel";
import clientApi from "@/services/apiClient/clientApiClient";
import { getFreeSubdomain } from "@/hooks/useTunnels";
import { isReservedDomain, getReservedDomainError } from "@/config/domains";

interface TunnelDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  tunnel?: Tunnel | null;
  onSuccess: () => void;
  existingTunnels?: Tunnel[];
}

const formSchema = z.object({
  domain: z.string().min(1, "Domain is required"),
  target_port: z.number().min(1).max(65535),
  is_enabled: z.boolean(),
});

type DomainType = "free" | "custom";

export default function TunnelDialog({
  open,
  onOpenChange,
  tunnel,
  onSuccess,
  existingTunnels = [],
}: TunnelDialogProps) {
  const [domainType, setDomainType] = useState<DomainType>("free");
  const [freeSubdomain, setFreeSubdomain] = useState<string>("");
  const [freeSubdomainAvailable, setFreeSubdomainAvailable] = useState<boolean>(true);
  const [loadingFreeSubdomain, setLoadingFreeSubdomain] = useState(false);
  const [freeSubdomainError, setFreeSubdomainError] = useState<string>("");
  const [isSubmitting, setIsSubmitting] = useState(false);

  const form = useForm<z.infer<typeof formSchema>>({
    resolver: zodResolver(formSchema),
    defaultValues: {
      domain: "",
      target_port: 80,
      is_enabled: true,
    },
  });

  // Reset state when dialog opens
  useEffect(() => {
    if (open) {
      if (tunnel) {
        // Editing existing tunnel
        form.reset({
          domain: tunnel.domain,
          target_port: tunnel.target_port,
          is_enabled: tunnel.is_enabled,
        });
        setDomainType("custom"); // Treat existing as custom for UI purposes (readonly)
      } else {
        // Creating new tunnel
        setDomainType("free");
        setFreeSubdomain("");
        setFreeSubdomainAvailable(true);
        setFreeSubdomainError("");
        form.reset({
          domain: "",
          target_port: 80,
          is_enabled: true,
        });
        loadFreeSubdomain();
      }
    }
  }, [open, tunnel, form]);

  const loadFreeSubdomain = async () => {
    setLoadingFreeSubdomain(true);
    setFreeSubdomainError("");
    try {
      const response = await getFreeSubdomain();
      setFreeSubdomain(response.domain);
      setFreeSubdomainAvailable(response.available);

      if (!response.available) {
        setDomainType("custom");
        form.setValue("domain", "");
      } else {
        form.setValue("domain", response.domain);
      }
    } catch (error: unknown) {
      const errorMessage = error instanceof Error ? error.message : "Failed to load free subdomain";
      setFreeSubdomainError(errorMessage);
    } finally {
      setLoadingFreeSubdomain(false);
    }
  };

  const handleDomainTypeChange = (value: string) => {
    if (!value) return;
    const newType = value as DomainType;
    setDomainType(newType);

    if (newType === "free" && freeSubdomain) {
      form.setValue("domain", freeSubdomain);
    } else if (newType === "custom") {
      form.setValue("domain", "");
    }
  };

  async function onSubmit(values: z.infer<typeof formSchema>) {
    // Custom validation
    if (domainType === "custom" && isReservedDomain(values.domain)) {
      form.setError("domain", { message: getReservedDomainError(values.domain) });
      return;
    }

    // Check for duplicate port
    const duplicateTunnel = existingTunnels.find(
      (t) => t.target_port === values.target_port && (!tunnel || t.id !== tunnel.id)
    );

    if (duplicateTunnel) {
      form.setError("target_port", {
        message: `Port ${values.target_port} is already used by another tunnel (${duplicateTunnel.domain})`
      });
      return;
    }

    setIsSubmitting(true);

    try {
      if (tunnel) {
        await clientApi().put<Tunnel>(`/tunnels/${tunnel.id}`, values);
        toast.success("Tunnel updated successfully");
      } else {
        const response = await clientApi().post<TunnelCreateResponse>("/tunnels", values);
        toast.success("Tunnel created successfully");

        if (response.token) {
          toast.success("Token copied to clipboard", {
            description: `Token: ${response.token.substring(0, 20)}...`,
          });
          navigator.clipboard.writeText(response.token);
        }
      }

      onSuccess();
      onOpenChange(false);
    } catch (error) {
      console.error("Error saving tunnel:", error);
      toast.error("Failed to save tunnel");
    } finally {
      setIsSubmitting(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[500px]">
        <DialogHeader>
          <DialogTitle>{tunnel ? "Edit Tunnel" : "Create New Tunnel"}</DialogTitle>
          <DialogDescription>
            Configure your tunnel settings. {tunnel ? "Domain cannot be changed." : "Choose a domain and local port."}
          </DialogDescription>
        </DialogHeader>

        {!tunnel && (
          <Alert className="bg-muted/50">
            <AlertDescription>
              <p>
                GiraffeCloud supports <strong>HTTP/HTTPS</strong> and <strong>WebSocket</strong> traffic.
              </p>
            </AlertDescription>
          </Alert>
        )}

        <Form {...form}>
          <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-6">
            {!tunnel && (
              <div className="space-y-3">
                <FormLabel>Domain Type</FormLabel>
                <ToggleGroup type="single" value={domainType} onValueChange={handleDomainTypeChange} className="justify-start">
                  <ToggleGroupItem value="free" aria-label="Free Subdomain" disabled={!freeSubdomainAvailable} className="flex-1">
                    <Wand2 className="mr-2 h-4 w-4" />
                    Free Subdomain
                  </ToggleGroupItem>
                  <ToggleGroupItem value="custom" aria-label="Custom Domain" className="flex-1">
                    <Globe className="mr-2 h-4 w-4" />
                    Custom Domain
                  </ToggleGroupItem>
                </ToggleGroup>

                {!freeSubdomainAvailable && freeSubdomain && (
                  <Alert variant="default" className="bg-blue-50 text-blue-900 border-blue-200 dark:bg-blue-950 dark:text-blue-100 dark:border-blue-800">
                    <AlertDescription>
                      You already have a free subdomain: <strong>{freeSubdomain}</strong>To create additional tunnels, please use a custom domain.
                    </AlertDescription>
                  </Alert>
                )}
              </div>
            )}

            {/* Free Subdomain Display */}
            {!tunnel && domainType === "free" && (
              <div className="space-y-2">
                {loadingFreeSubdomain ? (
                  <div className="flex items-center gap-2 text-sm text-muted-foreground">
                    <Loader2 className="h-4 w-4 animate-spin" />
                    Loading your free subdomain...
                  </div>
                ) : freeSubdomainError ? (
                  <Alert variant="destructive">
                    <AlertTriangle className="h-4 w-4" />
                    <AlertTitle>Error</AlertTitle>
                    <AlertDescription>{freeSubdomainError}</AlertDescription>
                  </Alert>
                ) : freeSubdomain ? (
                  <div className="p-4 border rounded-lg bg-muted/30">
                    <p className="text-sm text-muted-foreground mb-2">Your free subdomain:</p>
                    <Badge variant="secondary" className="text-base font-mono px-3 py-1">
                      {freeSubdomain}
                    </Badge>
                    <p className="text-xs text-muted-foreground mt-2">
                      This subdomain is uniquely generated for you.
                    </p>
                  </div>
                ) : null}
              </div>
            )}

            {/* Custom Domain Input */}
            {(tunnel || domainType === "custom") && (
              <FormField
                control={form.control}
                name="domain"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Domain</FormLabel>
                    <FormControl>
                      <Input
                        placeholder="example.com"
                        {...field}
                        disabled={!!tunnel}
                      />
                    </FormControl>
                    <FormDescription>
                      {tunnel ? "Domain cannot be changed after creation" : "Enter your custom domain (subdomains are also supported)"}.
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            )}

              <FormField
                control={form.control}
                name="target_port"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Target Port</FormLabel>
                    <FormControl>
                      <Input
                        type="number"
                        {...field}
                        onChange={(e) => field.onChange(e.target.valueAsNumber)}
                      />
                    </FormControl>
                    <FormDescription>
                      Port on your local machine to forward traffic to (1-65535).
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

            {tunnel && (
              <FormField
                control={form.control}
                name="is_enabled"
                render={({ field }) => (
                  <FormItem className="flex flex-row items-center justify-between rounded-lg border p-4">
                    <div className="space-y-0.5">
                      <FormLabel className="text-base">Enabled</FormLabel>
                      <FormDescription>
                        Enable or disable traffic forwarding for this tunnel.
                      </FormDescription>
                    </div>
                    <FormControl>
                      <Switch
                        checked={field.value}
                        onCheckedChange={field.onChange}
                      />
                    </FormControl>
                  </FormItem>
                )}
              />
            )}

            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
                Cancel
              </Button>
              <Button type="submit" disabled={isSubmitting || (domainType === "free" && !freeSubdomain && !tunnel)}>
                {isSubmitting && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                {tunnel ? "Update Tunnel" : "Create Tunnel"}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  );
}
