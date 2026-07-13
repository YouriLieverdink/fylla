<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

return new class extends Migration
{
    public function up(): void
    {
        Schema::create('projects', function (Blueprint $table) {
            $table->id();

            // Kendo-mirror fields (owned upstream, overwritten every sync).
            $table->unsignedBigInteger('kendo_id')->unique();
            $table->string('name');
            $table->string('code')->nullable();
            $table->timestamp('synced_at')->nullable();

            // Fylla-owned field (ADR-0004): the billable-projects list. Never
            // written by sync; drives worklog billability classification.
            $table->boolean('billable')->default(false);
        });
    }

    public function down(): void
    {
        Schema::dropIfExists('projects');
    }
};
