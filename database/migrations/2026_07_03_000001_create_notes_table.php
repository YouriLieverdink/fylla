<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

return new class extends Migration
{
    public function up(): void
    {
        // A note belongs to the open segment and rides into that segment's
        // Worklog comment (ADR-0005). created_at is the wall-clock stamp shown
        // as HH:MM; managed by hand (no updated_at).
        Schema::create('notes', function (Blueprint $table) {
            $table->id();
            $table->foreignId('segment_id')->constrained()->cascadeOnDelete();
            $table->timestamp('created_at');
            $table->text('text');
        });
    }

    public function down(): void
    {
        Schema::dropIfExists('notes');
    }
};
