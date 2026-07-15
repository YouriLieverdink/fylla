<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

return new class extends Migration
{
    public function up(): void
    {
        // Fylla-owned client entity (ADR-0011): groups Kendo projects. A project
        // assigned to a client is "managed" — its worklogs sync for the whole
        // team. Edited via UI (#21), never mirrored from Kendo.
        Schema::create('clients', function (Blueprint $table) {
            $table->id();
            $table->string('name');
            $table->unsignedInteger('monthly_target_hours')->nullable();
        });
    }

    public function down(): void
    {
        Schema::dropIfExists('clients');
    }
};
