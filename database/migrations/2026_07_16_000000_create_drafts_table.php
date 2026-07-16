<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

/**
 * Fylla-native drafts (ADR-0012): a third work source, owned entirely by Fylla.
 * No `kendo_id` — never synced to/from a provider, so the Kendo reconciliation
 * that deletes absent issues can't touch this table. Carries only the
 * schedulable fields the worklist scorer reads (ADR-0013).
 */
return new class extends Migration
{
    public function up(): void
    {
        Schema::create('drafts', function (Blueprint $table) {
            $table->id();
            $table->string('title');
            $table->string('priority')->default('Medium');
            $table->date('due_date')->nullable();
            $table->date('not_before')->nullable();
            $table->boolean('up_next')->nullable();
            $table->timestamps();
        });
    }

    public function down(): void
    {
        Schema::dropIfExists('drafts');
    }
};
