<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

return new class extends Migration
{
    public function up(): void
    {
        Schema::create('segments', function (Blueprint $table) {
            $table->id();
            $table->foreignId('timer_id')->constrained();
            $table->timestamp('started_at');
            $table->timestamp('ended_at')->nullable();
            $table->text('comment')->nullable();
        });
    }

    public function down(): void
    {
        Schema::dropIfExists('segments');
    }
};
